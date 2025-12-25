package providers

import (
	"encoding/json"
	"fmt"
	"github.com/yuriiter/trips/pkg/models"
	"github.com/yuriiter/trips/pkg/utils"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FlixbusProvider struct{}

func (f *FlixbusProvider) Name() string { return "Flixbus" }

func (f *FlixbusProvider) searchCityAutocomplete(name string) (*models.Location, error) {
	utils.DebugLog("Flixbus: Autocomplete search for '%s'", name)
	u := fmt.Sprintf("https://global.api.flixbus.com/search/autocomplete/cities?q=%s&lang=en&country=en&flixbus_cities_only=true&stations=true", url.QueryEscape(name))

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list []map[string]interface{}
	if err := json.Unmarshal(body, &list); err == nil {
		if len(list) > 0 {
			return &models.Location{
				ID:   fmt.Sprintf("%v", list[0]["id"]),
				Name: fmt.Sprintf("%v", list[0]["name"]),
			}, nil
		}
		return nil, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		if items, ok := obj["items"].([]interface{}); ok && len(items) > 0 {
			item := items[0].(map[string]interface{})
			return &models.Location{
				ID:   fmt.Sprintf("%v", item["id"]),
				Name: fmt.Sprintf("%v", item["name"]),
			}, nil
		}
	}

	utils.DebugLog("Flixbus: Failed to parse body: %s", string(body))
	return nil, fmt.Errorf("failed to parse flixbus autocomplete response")
}

func (f *FlixbusProvider) SearchLocationByName(name string) (*models.Location, error) {
	return f.searchCityAutocomplete(name)
}

func (f *FlixbusProvider) getCitiesInBbox(bbox map[string]map[string]float64) ([]models.Location, error) {
	bboxJson, _ := json.Marshal(bbox)
	u := fmt.Sprintf("https://global.api.flixbus.com/cms/cities?language=en&limit=5000&geo_bounding_box=%s", url.QueryEscape(string(bboxJson)))

	utils.DebugLog("Flixbus: Fetching cities in bbox")

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result []struct {
			ID      string `json:"uuid"`
			Name    string `json:"name"`
			Country string `json:"country"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var locs []models.Location
	for _, r := range result.Result {
		locs = append(locs, models.Location{ID: r.ID, Name: r.Name, Country: r.Country})
	}
	return locs, nil
}

func (f *FlixbusProvider) GetLocationsByCountry(countryCode string) ([]models.Location, error) {
	names := utils.GetCountryNamesByCode(countryCode)
	if len(names) == 0 {
		return nil, fmt.Errorf("unknown country code")
	}
	bbox, err := utils.GetCountryBoundingBox(names[0])
	if err != nil {
		return nil, err
	}
	cities, err := f.getCitiesInBbox(bbox)
	if err != nil {
		return nil, err
	}
	var filtered []models.Location
	for _, c := range cities {
		if strings.EqualFold(c.Country, countryCode) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func (f *FlixbusProvider) SearchLocationsByDistance(originName string, radiusKm int) ([]models.Location, error) {
	lat, lon, err := utils.GetCityCoordinates(originName)
	if err != nil {
		return nil, err
	}

	deltaLat := float64(radiusKm) / 111.0
	deltaLon := float64(radiusKm) / (111.0 * math.Cos(lat*math.Pi/180.0))

	bbox := map[string]map[string]float64{
		"top_left":     {"lat": lat + deltaLat, "lon": lon - deltaLon},
		"bottom_right": {"lat": lat - deltaLat, "lon": lon + deltaLon},
	}

	return f.getCitiesInBbox(bbox)
}

func (f *FlixbusProvider) SearchTrips(fromLoc, toLoc models.Location, date time.Time) ([]models.Trip, error) {
	dateStr := date.Format("02.01.2006")
	u := fmt.Sprintf("https://global.api.flixbus.com/search/service/v4/search?from_city_id=%s&to_city_id=%s&departure_date=%s&products=%%7B%%22adult%%22:1%%7D&currency=EUR&locale=en&search_by=cities&include_after_midnight_rides=1", fromLoc.ID, toLoc.ID, dateStr)

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
		Trips []struct {
			Results map[string]struct {
				Price struct {
					Total    float64 `json:"total"`
					Currency string  `json:"currency"`
				} `json:"price"`
				Duration struct {
					Hours   int `json:"hours"`
					Minutes int `json:"minutes"`
				} `json:"duration"`
				Departure struct {
					Date      string      `json:"date"`
					StationID interface{} `json:"station_id"`
				} `json:"departure"`
				Arrival struct {
					Date      string      `json:"date"`
					StationID interface{} `json:"station_id"`
				} `json:"arrival"`
				TransferType string `json:"transfer_type"`
			} `json:"results"`
		} `json:"trips"`
		Stations map[string]struct {
			Name string `json:"name"`
		} `json:"stations"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, err
	}

	var trips []models.Trip
	if len(response.Trips) == 0 {
		return trips, nil
	}

	for _, result := range response.Trips[0].Results {
		depTime, err := time.Parse(time.RFC3339, result.Departure.Date)
		if err != nil {
			continue
		}
		if depTime.Format("2006-01-02") != date.Format("2006-01-02") {
			continue
		}
		arrTime, _ := time.Parse(time.RFC3339, result.Arrival.Date)

		getStationID := func(v interface{}) string { return fmt.Sprintf("%v", v) }
		originName := "Unknown"
		if st, ok := response.Stations[getStationID(result.Departure.StationID)]; ok {
			originName = st.Name
		}
		destName := "Unknown"
		if st, ok := response.Stations[getStationID(result.Arrival.StationID)]; ok {
			destName = st.Name
		}

		transfers := 1
		if result.TransferType == "Direct" {
			transfers = 0
		}

		trips = append(trips, models.Trip{
			Provider:           "Flixbus",
			DepartureTime:      depTime,
			ArrivalTime:        arrTime,
			Duration:           fmt.Sprintf("%02dh %02dm", result.Duration.Hours, result.Duration.Minutes),
			Price:              result.Price.Total,
			Currency:           "EUR",
			OriginStation:      originName,
			DestinationStation: destName,
			Transfers:          transfers,
			VehicleType:        "BUS",
		})
	}
	return trips, nil
}
