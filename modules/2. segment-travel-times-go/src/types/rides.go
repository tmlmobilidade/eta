package types

type Ride struct {
	ID                 string   `bson:"_id"`
	HashedShapeID      string   `bson:"hashed_shape_id"`
	LineID             int      `bson:"line_id"`
	StartTimeScheduled int64    `bson:"start_time_scheduled"`
	TripID             string   `bson:"trip_id"`
	VehicleIDs         []uint32 `bson:"vehicle_ids"`
}