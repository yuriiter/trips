package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/yuriiter/trips/pkg/models"
	"github.com/yuriiter/trips/pkg/providers"
	"github.com/yuriiter/trips/pkg/utils"
)

var (
	fromArg   string
	toArg     string
	dateArg   string
	distArg   int
	provArg   string
	outArg    string
	sortArg   string
	debugFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "tripsearch",
	Short: "Search for bus/train trips",
	Run: func(cmd *cobra.Command, args []string) {
		utils.SetDebug(debugFlag)
		runSearch()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&fromArg, "from", "f", "", "Origin city or country")
	rootCmd.Flags().StringVarP(&toArg, "to", "t", "", "Destination city or country")
	rootCmd.Flags().StringVarP(&dateArg, "date", "d", "tomorrow", "Date (YYYY-MM-DD, today, tomorrow)")
	rootCmd.Flags().IntVarP(&distArg, "distance", "D", 0, "Search destinations within X km of origin")
	rootCmd.Flags().StringVarP(&provArg, "provider", "p", "all", "Provider (all, flixbus, regiojet)")
	rootCmd.Flags().StringVarP(&outArg, "out", "o", "", "Output file path (CSV)")
	rootCmd.Flags().StringVarP(&sortArg, "sort", "s", "price", "Sort by: price, departure")

	rootCmd.Flags().BoolVarP(&debugFlag, "debug", "v", false, "Enable debug logs")
	rootCmd.MarkFlagRequired("from")
}

func runSearch() {
	dates, err := utils.ParseDates(dateArg)
	if err != nil {
		fmt.Printf("Date error: %v\n", err)
		os.Exit(1)
	}

	var pList []providers.Provider
	if provArg == "flixbus" || provArg == "all" {
		pList = append(pList, &providers.FlixbusProvider{})
	}
	if provArg == "regiojet" || provArg == "all" {
		pList = append(pList, &providers.RegiojetProvider{})
	}

	allTrips := []models.Trip{}
	var tripMutex sync.Mutex
	var wg sync.WaitGroup

	origins := strings.Split(fromArg, ",")

	for _, p := range pList {
		fmt.Printf("\n--- Searching on %s ---\n", p.Name())

		var fromLocs []models.Location
		for _, oName := range origins {
			oName = strings.TrimSpace(oName)
			if cc := utils.GetCountryCodeByName(oName); cc != "" {
				if distArg > 0 {
					fmt.Printf("Warning: distance search not supported with country origin %s\n", oName)
					continue
				}
				fmt.Printf("Expanding origin country %s...\n", oName)
				locs, err := p.GetLocationsByCountry(cc)
				if err == nil {
					fromLocs = append(fromLocs, locs...)
				}
			} else {
				loc, err := p.SearchLocationByName(oName)
				if err == nil && loc != nil {
					fromLocs = append(fromLocs, *loc)
				} else {
					fmt.Printf("Warning: Origin '%s' not found on %s (Error: %v)\n", oName, p.Name(), err)
				}
			}
		}

		uniqueFrom := make(map[string]models.Location)
		for _, l := range fromLocs {
			uniqueFrom[l.ID] = l
		}

		for _, from := range uniqueFrom {
			fmt.Printf("\nSearching trips from: %s\n", from.Name)

			var destLocs []models.Location
			if distArg > 0 {
				fmt.Printf("Finding destinations within %dkm...\n", distArg)
				locs, err := p.SearchLocationsByDistance(from.Name, distArg)
				if err != nil {
					utils.DebugLog("Distance search error: %v", err)
				}
				destLocs = append(destLocs, locs...)
			} else if toArg != "" {
				toNames := strings.Split(toArg, ",")
				for _, tName := range toNames {
					tName = strings.TrimSpace(tName)
					if cc := utils.GetCountryCodeByName(tName); cc != "" {
						fmt.Printf("Expanding destination country %s...\n", tName)
						locs, err := p.GetLocationsByCountry(cc)
						if err == nil {
							destLocs = append(destLocs, locs...)
						}
					} else {
						loc, err := p.SearchLocationByName(tName)
						if err == nil && loc != nil {
							destLocs = append(destLocs, *loc)
						}
					}
				}
			} else {
				fmt.Println("Error: --to or --distance required")
				os.Exit(1)
			}

			uniqueDest := make(map[string]models.Location)
			for _, l := range destLocs {
				if l.ID != from.ID {
					uniqueDest[l.ID] = l
				}
			}

			fmt.Printf("Found %d unique destinations. Searching on %d dates...\n", len(uniqueDest), len(dates))

			sem := make(chan struct{}, 8)

			for _, dest := range uniqueDest {
				for _, d := range dates {
					wg.Add(1)
					sem <- struct{}{}
					go func(f, t models.Location, dt time.Time, prov providers.Provider) {
						defer wg.Done()
						defer func() { <-sem }()

						trips, err := prov.SearchTrips(f, t, dt)
						if err != nil {
							utils.DebugLog("Error searching %s->%s: %v", f.Name, t.Name, err)
							return
						}
						if len(trips) > 0 {
							tripMutex.Lock()
							allTrips = append(allTrips, trips...)
							tripMutex.Unlock()
							fmt.Print(".")
						}
					}(from, dest, d, p)
				}
			}
			wg.Wait()
		}
	}

	if len(allTrips) == 0 {
		fmt.Println("\nNo trips found.")
		return
	}

	sort.Slice(allTrips, func(i, j int) bool {
		if sortArg == "departure" {
			return allTrips[i].DepartureTime.Before(allTrips[j].DepartureTime)
		}
		return allTrips[i].Price < allTrips[j].Price
	})

	printTrips(allTrips)
	saveAndOpen(allTrips)
}

func printTrips(trips []models.Trip) {
	fmt.Printf("\n--- Found %d trips ---\n", len(trips))
	fmt.Printf("%-10s | %-16s | %-16s | %-6s | %-8s | %s -> %s\n", "Provider", "Dep", "Arr", "Price", "Dur", "Origin", "Dest")
	for _, t := range trips {
		fmt.Printf("%-10s | %-16s | %-16s | %5.2f%s | %-8s | %s -> %s\n",
			t.Provider,
			t.DepartureTime.Format("02.01 15:04"),
			t.ArrivalTime.Format("02.01 15:04"),
			t.Price, t.Currency,
			t.Duration,
			t.OriginStation,
			t.DestinationStation,
		)
	}
}

func saveAndOpen(trips []models.Trip) {
	savePath := outArg
	if savePath == "" {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, "trips")
		os.MkdirAll(dir, 0755)

		destName := "any"
		if toArg != "" {
			destName = strings.ReplaceAll(toArg, " ", "_")
		}
		if distArg > 0 {
			destName = fmt.Sprintf("%dkm", distArg)
		}

		fname := fmt.Sprintf("%s_%s_%s.csv",
			strings.ReplaceAll(fromArg, " ", "_"),
			destName,
			time.Now().Format("20060102_150405"))
		savePath = filepath.Join(dir, fname)
	}

	f, err := os.Create(savePath)
	if err == nil {
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write([]string{"Provider", "Departure", "Arrival", "Price", "Currency", "Duration", "Origin", "Destination", "Transfers", "Vehicle"})
		for _, t := range trips {
			w.Write([]string{
				t.Provider,
				t.DepartureTime.Format(time.RFC3339),
				t.ArrivalTime.Format(time.RFC3339),
				fmt.Sprintf("%.2f", t.Price),
				t.Currency,
				t.Duration,
				t.OriginStation,
				t.DestinationStation,
				fmt.Sprintf("%d", t.Transfers),
				t.VehicleType,
			})
		}
		w.Flush()
		fmt.Printf("\nSaved to %s\n", savePath)

		if outArg == "" {
			if path, err := exec.LookPath("tabview"); err == nil {
				fmt.Println("Opening tabview...")
				cmd := exec.Command(path, savePath)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			}
		}
	} else {
		fmt.Printf("Error saving file: %v\n", err)
	}
}
