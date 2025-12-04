package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yuriiter/trips/pkg/utils"
)

var (
	fromArg   string
	toArg     string
	dateArg   string
	debugFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "tripsearch",
	Short: "Search for bus/train trips",
	Run: func(cmd *cobra.Command, args []string) {
		utils.SetDebug(debugFlag)
		fmt.Println("Search logic not implemented yet")
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
	rootCmd.Flags().StringVarP(&dateArg, "date", "d", "tomorrow", "Date")
	rootCmd.Flags().BoolVarP(&debugFlag, "debug", "v", false, "Enable debug logs")
	rootCmd.MarkFlagRequired("from")
}
