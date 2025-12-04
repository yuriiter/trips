package models

import "time"

type Location struct {
	ID        string
	Name      string
	Country   string
	Latitude  float64
	Longitude float64
}

type Trip struct {
	Provider           string
	DepartureTime      time.Time
	ArrivalTime        time.Time
	Duration           string
	Price              float64
	Currency           string
	OriginStation      string
	DestinationStation string
	Transfers          int
	VehicleType        string
}
