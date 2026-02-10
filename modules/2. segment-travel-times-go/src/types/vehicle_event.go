package types

type VehicleEvent struct {
	Id string `ch:"_id"`
	AgencyId string `ch:"agency_id"`
	CreatedAt uint64 `ch:"created_at"`
	Geohash string `ch:"geohash"`
	HashedShapeId string `ch:"hashed_shape_id"`
	Latitude float64 `ch:"latitude"`
	LineId uint16 `ch:"line_id"`
	Longitude float64 `ch:"longitude"`
	RideId string `ch:"ride_id"`
	VehicleId string `ch:"vehicle_id"`
}