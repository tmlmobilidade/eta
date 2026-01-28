package processor

import (
	"testing"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

func TestGroupEventsByTrip(t *testing.T) {
	events := []types.VehicleEvent{
		{TripOperationalID: "trip1", CreatedAt: 1000},
		{TripOperationalID: "trip1", CreatedAt: 2000},
		{TripOperationalID: "trip2", CreatedAt: 1500},
		{TripOperationalID: "trip1", CreatedAt: 3000},
		{TripOperationalID: "trip2", CreatedAt: 2500},
	}

	grouped := groupEventsByTrip(events)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(grouped))
	}

	if len(grouped["trip1"]) != 3 {
		t.Errorf("Expected 3 events in trip1, got %d", len(grouped["trip1"]))
	}

	if len(grouped["trip2"]) != 2 {
		t.Errorf("Expected 2 events in trip2, got %d", len(grouped["trip2"]))
	}
}

func TestExtractHour(t *testing.T) {
	tests := []struct {
		name        string
		timestampMs int64
		expected    int
	}{
		{
			name:        "Midnight UTC",
			timestampMs: 0, // 1970-01-01 00:00:00 UTC
			expected:    0,
		},
		{
			name:        "Noon UTC",
			timestampMs: 12 * 3600 * 1000, // 12:00:00 UTC
			expected:    12,
		},
		{
			name:        "11 PM UTC",
			timestampMs: 23 * 3600 * 1000, // 23:00:00 UTC
			expected:    23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHour(tt.timestampMs)
			if result != tt.expected {
				t.Errorf("extractHour(%d) = %d, expected %d", tt.timestampMs, result, tt.expected)
			}
		})
	}
}

func TestCalculateTravelTimeSamples(t *testing.T) {
	tests := []struct {
		name           string
		matches        []types.NodeEventMatch
		expectedLen    int
		checkFirstNode int
	}{
		{
			name:        "Empty matches",
			matches:     []types.NodeEventMatch{},
			expectedLen: 0,
		},
		{
			name: "Single match (not enough)",
			matches: []types.NodeEventMatch{
				{NodeIndex: 0, CreatedAt: 1000, Hour: 10},
			},
			expectedLen: 0,
		},
		{
			name: "Two consecutive nodes",
			matches: []types.NodeEventMatch{
				{NodeIndex: 0, CreatedAt: 1000, Hour: 10},
				{NodeIndex: 1, CreatedAt: 2000, Hour: 10}, // 1 second later
			},
			expectedLen:    1,
			checkFirstNode: 1,
		},
		{
			name: "Three nodes with gap",
			matches: []types.NodeEventMatch{
				{NodeIndex: 0, CreatedAt: 0, Hour: 10},
				{NodeIndex: 3, CreatedAt: 3000, Hour: 10}, // 3 seconds, 3 nodes = 1s each
			},
			expectedLen: 3, // Nodes 1, 2, 3
		},
		{
			name: "Non-advancing nodes (should be skipped)",
			matches: []types.NodeEventMatch{
				{NodeIndex: 5, CreatedAt: 1000, Hour: 10},
				{NodeIndex: 3, CreatedAt: 2000, Hour: 10}, // Going backwards
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTravelTimeSamples(tt.matches)
			if len(result) != tt.expectedLen {
				t.Errorf("calculateTravelTimeSamples() returned %d samples, expected %d", len(result), tt.expectedLen)
			}
			if tt.expectedLen > 0 && tt.checkFirstNode > 0 && len(result) > 0 {
				if result[0].NodeIndex != tt.checkFirstNode {
					t.Errorf("First sample node index = %d, expected %d", result[0].NodeIndex, tt.checkFirstNode)
				}
			}
		})
	}

	// Test time distribution
	t.Run("Time distribution", func(t *testing.T) {
		matches := []types.NodeEventMatch{
			{NodeIndex: 0, CreatedAt: 0, Hour: 10},
			{NodeIndex: 4, CreatedAt: 4000, Hour: 10}, // 4 seconds across 4 nodes = 1s each
		}
		result := calculateTravelTimeSamples(matches)
		if len(result) != 4 {
			t.Fatalf("Expected 4 samples, got %d", len(result))
		}
		for i, sample := range result {
			if sample.TravelTimeSeconds != 1.0 {
				t.Errorf("Sample %d travel time = %v, expected 1.0", i, sample.TravelTimeSeconds)
			}
		}
	})
}

func TestAggregateSamples(t *testing.T) {
	samples := []types.NodeTravelTimeSample{
		{NodeIndex: 0, Hour: 10, TravelTimeSeconds: 1.0},
		{NodeIndex: 0, Hour: 10, TravelTimeSeconds: 2.0},
		{NodeIndex: 0, Hour: 10, TravelTimeSeconds: 3.0},
		{NodeIndex: 0, Hour: 11, TravelTimeSeconds: 5.0},
		{NodeIndex: 1, Hour: 10, TravelTimeSeconds: 2.0},
	}

	result := aggregateSamples(samples)

	// Should have 3 unique keys
	if len(result) != 3 {
		t.Errorf("Expected 3 aggregated entries, got %d", len(result))
	}

	// Check aggregation for node 0, hour 10
	key := "0-10"
	if agg, ok := result[key]; ok {
		if agg.SampleCount != 3 {
			t.Errorf("Sample count for %s = %d, expected 3", key, agg.SampleCount)
		}
		if agg.TotalTravelTime != 6.0 {
			t.Errorf("Total travel time for %s = %v, expected 6.0", key, agg.TotalTravelTime)
		}
	} else {
		t.Errorf("Missing aggregation for key %s", key)
	}
}

func TestProcessShapeTravelTimes_EmptyInput(t *testing.T) {
	// Test with empty events
	result := processShapeTravelTimes(
		[]types.VehicleEvent{},
		[]types.Coordinate{{0, 0}, {0, 1}},
		1001,
		"shape1",
		90.0,
	)
	if len(result) != 0 {
		t.Errorf("Expected 0 records for empty events, got %d", len(result))
	}

	// Test with insufficient nodes
	result = processShapeTravelTimes(
		[]types.VehicleEvent{{TripOperationalID: "trip1", CreatedAt: 1000}},
		[]types.Coordinate{{0, 0}}, // Only one node
		1001,
		"shape1",
		90.0,
	)
	if len(result) != 0 {
		t.Errorf("Expected 0 records for single node, got %d", len(result))
	}
}
