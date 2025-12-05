package utils

import (
	"strings"
	"time"
)

func ParseDates(input string) ([]time.Time, error) {
	var dates []time.Time
	parts := strings.Split(input, ",")

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "today" {
			dates = append(dates, time.Now())
			continue
		}
		if p == "tomorrow" {
			dates = append(dates, time.Now().AddDate(0, 0, 1))
			continue
		}

		formats := []string{"2006-01-02", "02.01.2006", "02.01"}
		var parsed time.Time
		var err error

		success := false
		for _, f := range formats {
			parsed, err = time.Parse(f, p)
			if err == nil {
				if f == "02.01" {
					now := time.Now()
					parsed = parsed.AddDate(now.Year(), 0, 0)

					if parsed.Before(now.AddDate(0, 0, -2)) {
						parsed = parsed.AddDate(1, 0, 0)
					}
				}
				dates = append(dates, parsed)
				success = true
				break
			}
		}
		if !success {
			return nil, err
		}
	}
	return dates, nil
}

func GetCountryCodeByName(name string) string {
	m := map[string]string{
		"germany": "DE", "deutschland": "DE",
		"czech": "CZ", "czech republic": "CZ", "czechia": "CZ",
		"austria": "AT", "france": "FR", "poland": "PL",
		"slovakia": "SK", "hungary": "HU", "italy": "IT",
		"netherlands": "NL", "croatia": "HR",
	}
	return m[strings.ToLower(name)]
}

func GetCountryNamesByCode(code string) []string {
	code = strings.ToUpper(code)
	m := map[string]string{
		"DE": "Germany", "CZ": "Czech Republic",
		"AT": "Austria", "FR": "France", "PL": "Poland",
		"SK": "Slovakia", "HU": "Hungary", "IT": "Italy",
		"NL": "Netherlands", "HR": "Croatia",
	}
	if name, ok := m[code]; ok {
		return []string{name}
	}
	return []string{}
}
