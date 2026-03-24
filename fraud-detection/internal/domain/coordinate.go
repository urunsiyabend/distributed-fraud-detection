package domain

import (
	"fmt"
	"math"
)

type Coordinate struct {
	Lat float64
	Lng float64
}

func NewCoordinate(lat, lng float64) (Coordinate, error) {
	if lat < -90 || lat > 90 {
		return Coordinate{}, fmt.Errorf("latitude must be between -90 and 90, got %f", lat)
	}
	if lng < -180 || lng > 180 {
		return Coordinate{}, fmt.Errorf("longitude must be between -180 and 180, got %f", lng)
	}
	return Coordinate{Lat: lat, Lng: lng}, nil
}

// DistanceKm returns the great-circle distance in kilometers using the Haversine formula.
func (c Coordinate) DistanceKm(other Coordinate) float64 {
	const earthRadiusKm = 6371.0

	lat1 := degreesToRadians(c.Lat)
	lat2 := degreesToRadians(other.Lat)
	dLat := degreesToRadians(other.Lat - c.Lat)
	dLng := degreesToRadians(other.Lng - c.Lng)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180
}
