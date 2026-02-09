package types

// VehicleEvent represents a vehicle position event from ClickHouse.
type VehicleEvent struct {
	TripOperationalID string  `ch:"trip_operational_id"`
	Geohash           string  `ch:"geohash"`
	CreatedAt         int64   `ch:"created_at"`
	Latitude          float64 `ch:"latitude"`
	Longitude         float64 `ch:"longitude"`
}