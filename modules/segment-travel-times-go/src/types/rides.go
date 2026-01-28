package types

type Ride struct {
	_id string `bson:"_id"`
	hashedShapeID string `bson:"hashed_shape_id"`
	lineID int `bson:"line_id"`
	startTimeScheduled int64 `bson:"start_time_scheduled"`
	tripID string `bson:"trip_id"`
	vehicleIDs []string `bson:"vehicle_ids"`
}