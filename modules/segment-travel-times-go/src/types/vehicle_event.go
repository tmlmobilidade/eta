package types

type VehicleEvent struct {
	createdAt int64 `bson:"created_at"`
	geohash string `bson:"geohash"`
	latitude float64 `bson:"latitude"`
	longitude float64 `bson:"longitude"`
	operationalDate OperationalDate `bson:"operational_date"`
	tripOperationalID string `bson:"trip_operational_id"`
}