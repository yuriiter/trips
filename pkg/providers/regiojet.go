package providers

import (
	"encoding/json"
	"fmt"
	"github.com/yuriiter/trips/pkg/models"
	"github.com/yuriiter/trips/pkg/utils"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RegiojetProvider struct {
	locationsData []interface{}
	initOnce      sync.Once
}

func (r *RegiojetProvider) Name() string { return "Regiojet" }

func (r *RegiojetProvider) ensureData() error {
	var err error
	r.initOnce.Do(func() {
		utils.DebugLog("Regiojet: Fetching all locations...")
		resp, reqErr := http.Get("https://brn-ybus-pubapi.sa.cz/restapi/consts/locations")
		if reqErr != nil {
			err = reqErr
			return
		}
		defer resp.Body.Close()
		if decodeErr := json.NewDecoder(resp.Body).Decode(&r.locationsData); decodeErr != nil {
			err = decodeErr
		}
	})
	return err
}

func (r *RegiojetProvider) parseCity(data map[string]interface{}) *models.Location {
	var id string
	switch v := data["id"].(type) {
	case float64:
		id = fmt.Sprintf("%.0f", v)
	default:
		id = fmt.Sprintf("%v", v)
	}

	name := data["name"].(string)

	var lat, lon float64
	if stations, ok := data["stations"].([]interface{}); ok && len(stations) > 0 {
		s := stations[0].(map[string]interface{})
		if l, ok := s["latitude"].(float64); ok {
			lat = l
		}
		if l, ok := s["longitude"].(float64); ok {
			lon = l
		}
	}
	return &models.Location{ID: id, Name: name, Latitude: lat, Longitude: lon}
}
func (r *RegiojetProvider) SearchLocationByName(name string) (*models.Location, error) {
	if err := r.ensureData(); err != nil {
		return nil, err
	}

	searchLower := strings.ToLower(name)
	for _, country := range r.locationsData {
		cMap := country.(map[string]interface{})
		cities := cMap["cities"].([]interface{})
		for _, city := range cities {
			c := city.(map[string]interface{})
			cName := c["name"].(string)
			if strings.EqualFold(cName, searchLower) {
				return r.parseCity(c), nil
			}
			if aliases, ok := c["aliases"].([]interface{}); ok {
				for _, a := range aliases {
					if strings.EqualFold(a.(string), searchLower) {
						return r.parseCity(c), nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (r *RegiojetProvider) GetLocationsByCountry(countryCode string) ([]models.Location, error) {
	if err := r.ensureData(); err != nil {
		return nil, err
	}
	var locs []models.Location
	for _, country := range r.locationsData {
		cMap := country.(map[string]interface{})
		code := cMap["code"].(string)
		if strings.EqualFold(code, countryCode) {
			cities := cMap["cities"].([]interface{})
			for _, city := range cities {
				locs = append(locs, *r.parseCity(city.(map[string]interface{})))
			}
		}
	}
	return locs, nil
}

func (r *RegiojetProvider) SearchLocationsByDistance(originName string, radiusKm int) ([]models.Location, error) {
	if err := r.ensureData(); err != nil {
		return nil, err
	}
	originLat, originLon, err := utils.GetCityCoordinates(originName)
	if err != nil {
		return nil, err
	}

	var locs []models.Location
	seen := make(map[string]bool)

	for _, country := range r.locationsData {
		cMap := country.(map[string]interface{})
		cities := cMap["cities"].([]interface{})
		for _, cityData := range cities {
			city := r.parseCity(cityData.(map[string]interface{}))
			if seen[city.Name] {
				continue
			}
			if city.Latitude != 0 && city.Longitude != 0 {
				dist := utils.HaversineDistance(originLat, originLon, city.Latitude, city.Longitude)
				if dist <= float64(radiusKm) {
					locs = append(locs, *city)
				}
			}
			seen[city.Name] = true
		}
	}
	return locs, nil
}

func (r *RegiojetProvider) SearchTrips(fromLoc, toLoc models.Location, date time.Time) ([]models.Trip, error) {
	dateStr := date.Format("2006-01-02")
	u := fmt.Sprintf("https://brn-ybus-pubapi.sa.cz/restapi/routes/search/simple?tariffs=REGULAR&toLocationType=CITY&toLocationId=%s&fromLocationType=CITY&fromLocationId=%s&departureDate=%s&currency=EUR", toLoc.ID, fromLoc.ID, dateStr)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api error %d", resp.StatusCode)
	}

	var response struct {
		Routes []struct {
			DepartureTime  string   `json:"departureTime"`
			ArrivalTime    string   `json:"arrivalTime"`
			TravelTime     string   `json:"travelTime"`
			PriceFrom      float64  `json:"priceFrom"`
			TransfersCount int      `json:"transfersCount"`
			VehicleTypes   []string `json:"vehicleTypes"`
		} `json:"routes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	var trips []models.Trip
	for _, route := range response.Routes {
		depTime, err := time.Parse("2006-01-02T15:04:05.000-07:00", route.DepartureTime)
		if err != nil {
			utils.DebugLog("Error parsing time: %v", err)
			continue
		}

		if depTime.Format("2006-01-02") != dateStr {
			continue
		}
		arrTime, _ := time.Parse("2006-01-02T15:04:05.000-07:00", route.ArrivalTime)

		dur := route.TravelTime
		dur = strings.ReplaceAll(dur, "h", "")
		dur = strings.ReplaceAll(dur, "\u00a0", "")
		dur = strings.TrimSpace(strings.ReplaceAll(dur, " ", ""))
		parts := strings.Split(dur, ":")
		finalDur := route.TravelTime
		if len(parts) == 2 {
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			finalDur = fmt.Sprintf("%02dh %02dm", h, m)
		}

		trips = append(trips, models.Trip{
			Provider:           "Regiojet",
			DepartureTime:      depTime,
			ArrivalTime:        arrTime,
			Duration:           finalDur,
			Price:              route.PriceFrom,
			Currency:           "EUR",
			OriginStation:      fromLoc.Name,
			DestinationStation: toLoc.Name,
			Transfers:          route.TransfersCount,
			VehicleType:        strings.Join(route.VehicleTypes, ", "),
		})
	}
	return trips, nil
}
