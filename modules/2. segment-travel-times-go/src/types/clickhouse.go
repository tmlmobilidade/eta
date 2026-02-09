package types

// ClickhouseClientParams contains the configuration for connecting to ClickHouse.
type ClickhouseClientParams struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

// NodeTravelTimeRecord represents a travel time record for ClickHouse storage.
type NodeTravelTimeRecord struct {
	LineID            uint32  `ch:"line_id"`
	HashedShapeID     string  `ch:"hashed_shape_id"`
	NodeIndex         uint16  `ch:"node_index"`
	Latitude          float64 `ch:"latitude"`
	Longitude         float64 `ch:"longitude"`
	Hour              uint8   `ch:"hour"`
	TravelTimeSeconds float32 `ch:"travel_time_seconds"`
	SampleCount       uint32  `ch:"sample_count"`
}
