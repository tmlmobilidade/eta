package types

type Coordinate [2]float64

// Longitude returns the longitude component of the coordinate.
func (c Coordinate) Longitude() float64 {
	return c[0]
}

// Latitude returns the latitude component of the coordinate.
func (c Coordinate) Latitude() float64 {
	return c[1]
}

type OperationalDate string // YYYYMMDD
type UnixTimestamp int64

// Settings contains configuration for the segment travel times calculation.
type Settings struct {
	// MinEvents is the minimum number of events required for a trip to be analyzed.
	MinEvents int
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
	// MaxNodeMatchDistanceMeters is the maximum allowed distance (in meters)
	// between a vehicle event and a segment node when matching events to nodes.
	// If set to a value <= 0, no distance limit is applied.
	MaxNodeMatchDistanceMeters float64
	// WorkerCount is the number of parallel workers for line processing.
	WorkerCount int
	// BatchSize is the number of lines to process per batch.
	// Between batches, the processor checks for cancellation and flushes results.
	BatchSize int
}