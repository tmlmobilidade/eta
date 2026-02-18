package types

import "sort"

// NodeTravelTimeRecord is the final output per shape node per hour of day.
type NodeTravelTimeRecord struct {
    ShapeID       	string `ch:"hashed_shape_id"`
    NodeIndex       int `ch:"node_index"`
    Hour          	uint8 `ch:"hour"`
    Latitude      	float64 `ch:"latitude"`
    Longitude     	float64 `ch:"longitude"`
    TravelTimeSeconds float32 `ch:"travel_time_seconds"`
    SampleCount   	uint32  `ch:"sample_count"`
}

// NodeHourKey uniquely identifies a node within a shape at a specific hour.
type NodeHourKey struct {
    NodeIdx int
    Hour    uint8
}

// NodeAccumulator collects travel time samples for median calculation.
type NodeAccumulator struct {
    Samples []float64
}

// Median returns the median travel time from collected samples.
func (a *NodeAccumulator) Median() float64 {
    n := len(a.Samples)
    if n == 0 {
        return 0
    }

    sorted := make([]float64, n)
    copy(sorted, a.Samples)
    sort.Float64s(sorted)

    if n%2 == 0 {
        return (sorted[n/2-1] + sorted[n/2]) / 2.0
    }
    return sorted[n/2]
}