package geo

import (
	"main/src/lib/geo"
	"math"
	"testing"

	"main/src/types"
)

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name     string
		from     types.Coordinate
		to       types.Coordinate
		expected float64
		delta    float64 // Allowed error in meters
	}{
		{
			name:     "Same point",
			from:     types.Coordinate{0, 0},
			to:       types.Coordinate{0, 0},
			expected: 0,
			delta:    0.001,
		},
		{
			name:     "Lisbon to Porto",
			from:     types.Coordinate{-9.139337, 38.722252}, // Lisbon
			to:       types.Coordinate{-8.611899, 41.149561}, // Porto
			expected: 274000,                                 // ~274 km
			delta:    5000,                                   // Allow 5km error
		},
		{
			name:     "Short distance (100m)",
			from:     types.Coordinate{-9.139337, 38.722252},
			to:       types.Coordinate{-9.138337, 38.722252}, // ~87m east
			expected: 87,
			delta:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.HaversineDistance(tt.from, tt.to)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("HaversineDistance(%v, %v) = %v meters, expected %v (±%v)", tt.from, tt.to, result, tt.expected, tt.delta)
			}
		})
	}
}

func TestFindNearestNodeIndex(t *testing.T) {
	nodes := []types.Coordinate{
		{0, 0},
		{0, 1},
		{1, 1},
		{1, 0},
	}

	tests := []struct {
		name     string
		lon      float64
		lat      float64
		expected int
	}{
		{
			name:     "Exact match first node",
			lon:      0,
			lat:      0,
			expected: 0,
		},
		{
			name:     "Exact match second node",
			lon:      0,
			lat:      1,
			expected: 1,
		},
		{
			name:     "Closer to third node",
			lon:      0.9,
			lat:      0.9,
			expected: 2,
		},
		{
			name:     "Equidistant (picks first found)",
			lon:      0.5,
			lat:      0.5,
			expected: 0, // or could be any, depending on iteration order
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.FindNearestNodeIndex(tt.lon, tt.lat, nodes)
			// For equidistant case, just check it's a valid index
			if tt.name == "Equidistant (picks first found)" {
				if result < 0 || result >= len(nodes) {
					t.Errorf("FindNearestNodeIndex() returned invalid index %d", result)
				}
			} else if result != tt.expected {
				t.Errorf("FindNearestNodeIndex(%v, %v) = %d, expected %d", tt.lon, tt.lat, result, tt.expected)
			}
		})
	}
}

func TestLineLength(t *testing.T) {
	tests := []struct {
		name     string
		coords   []types.Coordinate
		expected float64
		delta    float64
	}{
		{
			name:     "Empty",
			coords:   []types.Coordinate{},
			expected: 0,
			delta:    0.001,
		},
		{
			name:     "Single point",
			coords:   []types.Coordinate{{0, 0}},
			expected: 0,
			delta:    0.001,
		},
		{
			name:     "Two points",
			coords:   []types.Coordinate{{0, 0}, {0, 1}},
			expected: 111195, // ~111km for 1 degree latitude
			delta:    1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.LineLength(tt.coords)
			if math.Abs(result-tt.expected) > tt.delta {
				t.Errorf("LineLength() = %v, expected %v (±%v)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestChunkLineIntoSegments(t *testing.T) {
	// Create a simple line from (0,0) to (0,0.001) - roughly 111 meters
	coords := []types.Coordinate{
		{0, 0},
		{0, 0.001}, // ~111m north
	}

	tests := []struct {
		name          string
		coords        []types.Coordinate
		segmentLength float64
		minSegments   int
	}{
		{
			name:          "Empty coords",
			coords:        []types.Coordinate{},
			segmentLength: 50,
			minSegments:   0,
		},
		{
			name:          "Single point",
			coords:        []types.Coordinate{{0, 0}},
			segmentLength: 50,
			minSegments:   0,
		},
		{
			name:          "50m segments on 111m line",
			coords:        coords,
			segmentLength: 50,
			minSegments:   2, // At least 2 segments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.ChunkLineIntoSegments(tt.coords, tt.segmentLength)
			if len(result) < tt.minSegments {
				t.Errorf("ChunkLineIntoSegments() returned %d segments, expected at least %d", len(result), tt.minSegments)
			}
		})
	}

	// Verify that the last point is always the end of the line
	t.Run("Last point is line end", func(t *testing.T) {
		result := geo.ChunkLineIntoSegments(coords, 50)
		if len(result) > 0 {
			lastPoint := result[len(result)-1]
			expected := coords[len(coords)-1]
			if lastPoint != expected {
				t.Errorf("Last point %v should be line end %v", lastPoint, expected)
			}
		}
	})
}

func TestInterpolatePoint(t *testing.T) {
	coords := []types.Coordinate{
		{0, 0},
		{0, 0.001}, // ~111m north
	}

	tests := []struct {
		name     string
		distance float64
	}{
		{
			name:     "Start of line",
			distance: 0,
		},
		{
			name:     "Middle of line",
			distance: 50,
		},
		{
			name:     "Past end of line",
			distance: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.InterpolatePoint(coords, tt.distance)
			// Just verify we get a valid coordinate
			if math.IsNaN(result.Latitude()) || math.IsNaN(result.Longitude()) {
				t.Errorf("InterpolatePoint() returned NaN coordinate")
			}
		})
	}
}
