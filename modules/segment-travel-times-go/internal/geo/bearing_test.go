package geo

import (
	"math"
	"testing"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

func TestCalculateBearing(t *testing.T) {
	tests := []struct {
		name     string
		from     types.Coordinate
		to       types.Coordinate
		expected float64
		delta    float64
	}{
		{
			name:     "North",
			from:     types.Coordinate{0, 0},
			to:       types.Coordinate{0, 1},
			expected: 0,
			delta:    0.1,
		},
		{
			name:     "East",
			from:     types.Coordinate{0, 0},
			to:       types.Coordinate{1, 0},
			expected: 90,
			delta:    0.1,
		},
		{
			name:     "South",
			from:     types.Coordinate{0, 1},
			to:       types.Coordinate{0, 0},
			expected: 180,
			delta:    0.1,
		},
		{
			name:     "West",
			from:     types.Coordinate{1, 0},
			to:       types.Coordinate{0, 0},
			expected: 270,
			delta:    0.1,
		},
		{
			name:     "Northeast",
			from:     types.Coordinate{0, 0},
			to:       types.Coordinate{1, 1},
			expected: 45,
			delta:    1.0, // Less precise for diagonal
		},
		{
			name:     "Lisbon to Porto (roughly north)",
			from:     types.Coordinate{-9.139337, 38.722252}, // Lisbon
			to:       types.Coordinate{-8.611899, 41.149561}, // Porto
			expected: 9.3,                                    // Roughly north-northeast
			delta:    1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBearing(tt.from, tt.to)
			diff := math.Abs(result - tt.expected)
			// Handle wraparound at 360
			if diff > 180 {
				diff = 360 - diff
			}
			if diff > tt.delta {
				t.Errorf("CalculateBearing(%v, %v) = %v, expected %v (±%v)", tt.from, tt.to, result, tt.expected, tt.delta)
			}
		})
	}
}

func TestGetAngularDifference(t *testing.T) {
	tests := []struct {
		name     string
		bearing1 float64
		bearing2 float64
		expected float64
	}{
		{
			name:     "Same bearing",
			bearing1: 90,
			bearing2: 90,
			expected: 0,
		},
		{
			name:     "Simple difference",
			bearing1: 90,
			bearing2: 45,
			expected: 45,
		},
		{
			name:     "Wraparound at 0/360",
			bearing1: 350,
			bearing2: 10,
			expected: 20,
		},
		{
			name:     "Opposite directions",
			bearing1: 0,
			bearing2: 180,
			expected: 180,
		},
		{
			name:     "Near wraparound",
			bearing1: 5,
			bearing2: 355,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAngularDifference(tt.bearing1, tt.bearing2)
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("GetAngularDifference(%v, %v) = %v, expected %v", tt.bearing1, tt.bearing2, result, tt.expected)
			}
		})
	}
}

func TestIsValidBearing(t *testing.T) {
	tests := []struct {
		name          string
		eventBearing  float64
		shapeBearing  float64
		threshold     float64
		expectedValid bool
	}{
		{
			name:          "Same bearing",
			eventBearing:  90,
			shapeBearing:  90,
			threshold:     45,
			expectedValid: true,
		},
		{
			name:          "Within threshold",
			eventBearing:  85,
			shapeBearing:  90,
			threshold:     45,
			expectedValid: true,
		},
		{
			name:          "Outside threshold",
			eventBearing:  180,
			shapeBearing:  90,
			threshold:     45,
			expectedValid: false,
		},
		{
			name:          "Wraparound within threshold",
			eventBearing:  5,
			shapeBearing:  355,
			threshold:     45,
			expectedValid: true,
		},
		{
			name:          "Opposite direction",
			eventBearing:  270,
			shapeBearing:  90,
			threshold:     90,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidBearing(tt.eventBearing, tt.shapeBearing, tt.threshold)
			if result != tt.expectedValid {
				t.Errorf("IsValidBearing(%v, %v, %v) = %v, expected %v", tt.eventBearing, tt.shapeBearing, tt.threshold, result, tt.expectedValid)
			}
		})
	}
}

func TestCalculateShapeBearings(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []types.Coordinate
		expected int // Expected number of bearings
	}{
		{
			name:     "Empty nodes",
			nodes:    []types.Coordinate{},
			expected: 0,
		},
		{
			name:     "Single node",
			nodes:    []types.Coordinate{{0, 0}},
			expected: 0,
		},
		{
			name:     "Two nodes",
			nodes:    []types.Coordinate{{0, 0}, {0, 1}},
			expected: 2,
		},
		{
			name:     "Multiple nodes",
			nodes:    []types.Coordinate{{0, 0}, {0, 1}, {1, 1}, {1, 0}},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateShapeBearings(tt.nodes)
			if len(result) != tt.expected {
				t.Errorf("CalculateShapeBearings() returned %d bearings, expected %d", len(result), tt.expected)
			}
		})
	}

	// Test that last bearing equals previous bearing
	t.Run("Last bearing equals previous", func(t *testing.T) {
		nodes := []types.Coordinate{{0, 0}, {0, 1}, {1, 1}}
		result := CalculateShapeBearings(nodes)
		if len(result) < 2 {
			t.Fatal("Expected at least 2 bearings")
		}
		if result[len(result)-1] != result[len(result)-2] {
			t.Errorf("Last bearing (%v) should equal previous bearing (%v)", result[len(result)-1], result[len(result)-2])
		}
	})
}
