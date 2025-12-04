package providers

import (
	"github.com/yuriiter/trips/pkg/models"
	"time"
)

type Provider interface {
	Name() string
	SearchLocationByName(name string) (*models.Location, error)
	GetLocationsByCountry(countryCode string) ([]models.Location, error)
	SearchLocationsByDistance(originName string, radiusKm int) ([]models.Location, error)
	SearchTrips(fromLoc, toLoc models.Location, date time.Time) ([]models.Trip, error)
}
