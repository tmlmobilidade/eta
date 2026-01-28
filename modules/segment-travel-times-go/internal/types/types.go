// Package types defines all data structures used throughout the segment travel times processor.
package types

// Coordinate represents a geographic coordinate pair [longitude, latitude] in GeoJSON order.
type Coordinate [2]float64

// Longitude returns the longitude component of the coordinate.
func (c Coordinate) Longitude() float64 {
	return c[0]
}

// Latitude returns the latitude component of the coordinate.
func (c Coordinate) Latitude() float64 {
	return c[1]
}

// LineShapeData contains shape data associated with a single line.
// It groups all hashed shapes, their segment nodes, and unique geohashes.
type LineShapeData struct {
	// Geohashes is a set of unique geohashes covering all segment endpoints.
	Geohashes map[string]struct{}
	// HashedShapeIDs contains all hashed shape IDs belonging to this line.
	HashedShapeIDs []string
	// Nodes maps hashed_shape_id to its segment endpoint coordinates.
	Nodes map[string][]Coordinate
}

// NewLineShapeData creates an empty LineShapeData with initialized collections.
func NewLineShapeData() *LineShapeData {
	return &LineShapeData{
		Geohashes:      make(map[string]struct{}),
		HashedShapeIDs: []string{},
		Nodes:          make(map[string][]Coordinate),
	}
}

// VehicleEvent represents a cached vehicle event with geohash association.
type VehicleEvent struct {
	CreatedAt         int64   `json:"created_at" ch:"created_at"`
	Geohash           string  `json:"geohash" ch:"geohash"`
	Latitude          float64 `json:"latitude" ch:"latitude"`
	Longitude         float64 `json:"longitude" ch:"longitude"`
	OperationalDate   string  `json:"operational_date" ch:"operational_date"`
	TripOperationalID string  `json:"trip_operational_id" ch:"trip_operational_id"`
}

// Settings contains configuration for the segment travel times calculation.
type Settings struct {
	// BearingThreshold is the bearing threshold in degrees for filtering events (0-180).
	BearingThreshold float64
	// GeohashPrecision is the geohash precision level (typically 6-8).
	GeohashPrecision int
	// RideEndDate is the Unix timestamp for ride query end date.
	RideEndDate int64
	// RideStartDate is the Unix timestamp for ride query start date.
	RideStartDate int64
	// SegmentLengthMeters is the length of each segment in meters.
	SegmentLengthMeters float64
	// WorkerCount is the number of parallel workers for line processing.
	WorkerCount int
}

// RideProjection represents a ride document projection for the aggregation query.
type RideProjection struct {
	ID                 string   `bson:"_id"`
	HashedShapeID      string   `bson:"hashed_shape_id"`
	LineID             int      `bson:"line_id"`
	StartTimeScheduled int64    `bson:"start_time_scheduled"`
	TripID             string   `bson:"trip_id"`
	VehicleIDs         []int `bson:"vehicle_ids"`
}

// ShapePoint represents point data from a hashed shape.
type ShapePoint struct {
	ShapePtLat float64 `bson:"shape_pt_lat"`
	ShapePtLon float64 `bson:"shape_pt_lon"`
}

// HashedShapePointProjection represents a hashed shape with projected point fields.
type HashedShapePointProjection struct {
	ID     string       `bson:"_id"`
	Points []ShapePoint `bson:"points"`
}

// NodeEventMatch represents an event matched to a specific node index.
type NodeEventMatch struct {
	// CreatedAt is the Unix timestamp when the event occurred.
	CreatedAt int64
	// Hour is the hour of the day (0-23) extracted from CreatedAt.
	Hour int
	// NodeIndex is the index of the matched node in the shape.
	NodeIndex int
	// OperationalDate is the operational date of the event.
	OperationalDate string
}

// NodeTravelTimeSample represents a travel time sample for a node.
type NodeTravelTimeSample struct {
	// Hour is the hour of the day (0-23).
	Hour int
	// NodeIndex is the index of the node in the shape.
	NodeIndex int
	// TravelTimeSeconds is the travel time in seconds to reach this node.
	TravelTimeSeconds float64
}

// NodeTravelTimeRecord is the record structure for ClickHouse storage.
type NodeTravelTimeRecord struct {
	// LineID is the line ID.
	LineID int `json:"line_id" ch:"line_id"`
	// HashedShapeID is the hashed shape ID.
	HashedShapeID string `json:"hashed_shape_id" ch:"hashed_shape_id"`
	// NodeIndex is the index of the node in the shape.
	NodeIndex int `json:"node_index" ch:"node_index"`
	// Latitude is the latitude of the node.
	Latitude float64 `json:"latitude" ch:"latitude"`
	// Longitude is the longitude of the node.
	Longitude float64 `json:"longitude" ch:"longitude"`
	// Hour is the hour of the day (0-23).
	Hour int `json:"hour" ch:"hour"`
	// TravelTimeSeconds is the average travel time in seconds.
	TravelTimeSeconds float64 `json:"travel_time_seconds" ch:"travel_time_seconds"`
	// SampleCount is the number of samples.
	SampleCount int `json:"sample_count" ch:"sample_count"`
}

// LineEntry represents a line with its associated shape data for processing.
type LineEntry struct {
	LineID int
	Data   *LineShapeData
}

// AggregatedSample holds aggregated travel time data for a node by hour.
type AggregatedSample struct {
	Hour            int
	NodeIndex       int
	SampleCount     int
	TotalTravelTime float64
}
