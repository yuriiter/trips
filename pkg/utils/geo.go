package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	lat1 = lat1 * (math.Pi / 180.0)
	lat2 = lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func GetCityCoordinates(city string) (float64, float64, error) {
	DebugLog("Fetching coords for %s", city)
	u := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1", url.QueryEscape(city))

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "TripSearchCLI/1.0")

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}
	if len(result) == 0 {
		return 0, 0, fmt.Errorf("coordinates not found for %s", city)
	}

	var lat, lon float64
	fmt.Sscanf(result[0].Lat, "%f", &lat)
	fmt.Sscanf(result[0].Lon, "%f", &lon)

	return lat, lon, nil
}

func GetCountryBoundingBox(countryName string) (map[string]map[string]float64, error) {
	DebugLog("Using generic Europe BBox for country %s", countryName)
	return map[string]map[string]float64{
		"top_left":     {"lat": 71.0, "lon": -10.0},
		"bottom_right": {"lat": 35.0, "lon": 40.0},
	}, nil
}
