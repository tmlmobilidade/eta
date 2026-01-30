package processor

import (
	"main/src/lib/geo"
	"main/src/types"
	"math"
	"testing"
)

func TestGroupEventsByTrip(t *testing.T) {
	tests := []struct {
		name           string
		events         []types.VehicleEvent
		expectedTrips  int
		expectedCounts map[string]int
	}{
		{
			name:          "empty slice",
			events:        []types.VehicleEvent{},
			expectedTrips: 0,
		},
		{
			name: "single trip",
			events: []types.VehicleEvent{
				{TripOperationalID: "trip-1", CreatedAt: 1000},
				{TripOperationalID: "trip-1", CreatedAt: 2000},
				{TripOperationalID: "trip-1", CreatedAt: 3000},
			},
			expectedTrips:  1,
			expectedCounts: map[string]int{"trip-1": 3},
		},
		{
			name: "multiple trips",
			events: []types.VehicleEvent{
				{TripOperationalID: "trip-1", CreatedAt: 1000},
				{TripOperationalID: "trip-2", CreatedAt: 2000},
				{TripOperationalID: "trip-1", CreatedAt: 3000},
				{TripOperationalID: "trip-2", CreatedAt: 4000},
				{TripOperationalID: "trip-2", CreatedAt: 5000},
			},
			expectedTrips:  2,
			expectedCounts: map[string]int{"trip-1": 2, "trip-2": 3},
		},
		{
			name: "each event unique trip",
			events: []types.VehicleEvent{
				{TripOperationalID: "a"},
				{TripOperationalID: "b"},
				{TripOperationalID: "c"},
			},
			expectedTrips:  3,
			expectedCounts: map[string]int{"a": 1, "b": 1, "c": 1},
		},
		{
			name: "preserves event data",
			events: []types.VehicleEvent{
				{TripOperationalID: "trip-1", Latitude: 60.1, Longitude: 24.9, CreatedAt: 1000},
			},
			expectedTrips:  1,
			expectedCounts: map[string]int{"trip-1": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupEventsByTrip(tt.events)

			if len(result) != tt.expectedTrips {
				t.Errorf("expected %d trips, got %d", tt.expectedTrips, len(result))
			}

			for tripID, expectedCount := range tt.expectedCounts {
				if len(result[tripID]) != expectedCount {
					t.Errorf("trip %s: expected %d events, got %d", tripID, expectedCount, len(result[tripID]))
				}
			}
		})
	}

	// Verify data preservation separately
	t.Run("preserves all fields", func(t *testing.T) {
		events := []types.VehicleEvent{
			{TripOperationalID: "trip-1", Latitude: 60.1, Longitude: 24.9, CreatedAt: 1000, Geohash: "u4x"},
		}
		result := groupEventsByTrip(events)
		e := result["trip-1"][0]
		if e.Latitude != 60.1 || e.Longitude != 24.9 || e.CreatedAt != 1000 || e.Geohash != "u4x" {
			t.Errorf("event fields not preserved: %+v", e)
		}
	})
}

func TestExtractHour(t *testing.T) {
	tests := []struct {
		name     string
		inputMs  int64
		expected int
	}{
		{
			name:     "midnight UTC",
			inputMs:  0,
			expected: 0,
		},
		{
			name:     "1am UTC",
			inputMs:  3600000,
			expected: 1,
		},
		{
			name:     "noon UTC",
			inputMs:  43200000,
			expected: 12,
		},
		{
			name:     "11pm UTC",
			inputMs:  82800000,
			expected: 23,
		},
		{
			name:     "wraps around 24h",
			inputMs:  86400000,
			expected: 0,
		},
		{
			name:     "25th hour wraps to 1",
			inputMs:  90000000,
			expected: 1,
		},
		{
			name:     "arbitrary timestamp 2024-01-30 13:00 UTC",
			inputMs:  1706619600000,
			expected: 13,
		},
		{
			name:     "half-second precision ignored",
			inputMs:  3600500,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHour(tt.inputMs)
			if result != tt.expected {
				t.Errorf("extractHour(%d) = %d, want %d", tt.inputMs, result, tt.expected)
			}
		})
	}
}

func TestMatchEventsToNodes(t *testing.T) {
	// Create a simple north-south line (bearing ~0 degrees / north)
	// Nodes go from south to north
	nodes := []types.Coordinate{
		{24.9, 60.0},   // node 0 (lon, lat)
		{24.9, 60.001}, // node 1
		{24.9, 60.002}, // node 2
		{24.9, 60.003}, // node 3
		{24.9, 60.004}, // node 4
	}

	// Bearings for a north-south line are ~0 degrees (north)
	shapeBearings := []float64{0, 0, 0, 0, 0}
	threshold := 45.0

	// Create NodeIndex for main tests
	nodeIndex := geo.NewNodeIndex(nodes, 7)

	tests := []struct {
		name          string
		events        []types.VehicleEvent
		nodeIndex     *geo.NodeIndex
		bearings      []float64
		threshold     float64
		expectedCount int
	}{
		{
			name:          "empty events",
			events:        []types.VehicleEvent{},
			nodeIndex:     nodeIndex,
			bearings:      shapeBearings,
			threshold:     threshold,
			expectedCount: 0,
		},
		{
			name: "single event returns nil",
			events: []types.VehicleEvent{
				{Longitude: 24.9, Latitude: 60.0, CreatedAt: 1000},
			},
			nodeIndex:     nodeIndex,
			bearings:      shapeBearings,
			threshold:     threshold,
			expectedCount: 0,
		},
		{
			name:          "empty nodes",
			events:        []types.VehicleEvent{{}, {}},
			nodeIndex:     geo.NewNodeIndex([]types.Coordinate{}, 7),
			bearings:      []float64{},
			threshold:     threshold,
			expectedCount: 0,
		},
		{
			name:          "single node returns nil",
			events:        []types.VehicleEvent{{}, {}},
			nodeIndex:     geo.NewNodeIndex([]types.Coordinate{{24.9, 60.0}}, 7),
			bearings:      []float64{0},
			threshold:     threshold,
			expectedCount: 0,
		},
		{
			name: "two events traveling north on north-south shape",
			events: []types.VehicleEvent{
				{Longitude: 24.9, Latitude: 60.0, CreatedAt: 1000},
				{Longitude: 24.9, Latitude: 60.004, CreatedAt: 5000},
			},
			nodeIndex:     nodeIndex,
			bearings:      shapeBearings,
			threshold:     threshold,
			expectedCount: 2, // first event matched + last event advancing
		},
		{
			name: "two events traveling south on north-south shape filtered out",
			events: []types.VehicleEvent{
				{Longitude: 24.9, Latitude: 60.004, CreatedAt: 1000},
				{Longitude: 24.9, Latitude: 60.0, CreatedAt: 5000},
			},
			nodeIndex:     nodeIndex,
			bearings:      shapeBearings,
			threshold:     threshold,
			expectedCount: 0, // bearing ~180, threshold 45, filtered out
		},
		{
			name: "three events advancing along shape",
			events: []types.VehicleEvent{
				{Longitude: 24.9, Latitude: 60.0, CreatedAt: 1000},
				{Longitude: 24.9, Latitude: 60.002, CreatedAt: 3000},
				{Longitude: 24.9, Latitude: 60.004, CreatedAt: 5000},
			},
			nodeIndex:     nodeIndex,
			bearings:      shapeBearings,
			threshold:     threshold,
			expectedCount: 3, // 2 from pairs + 1 last event
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchEventsToNodes(tt.events, tt.nodeIndex, tt.bearings, tt.threshold)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d matches, got %d", tt.expectedCount, len(result))
			}
		})
	}

	// Verify last event not added when not advancing
	t.Run("last event not advancing is excluded", func(t *testing.T) {
		events := []types.VehicleEvent{
			{Longitude: 24.9, Latitude: 60.002, CreatedAt: 1000},
			{Longitude: 24.9, Latitude: 60.004, CreatedAt: 3000},
			{Longitude: 24.9, Latitude: 60.001, CreatedAt: 5000}, // goes backward
		}
		result := matchEventsToNodes(events, nodeIndex, shapeBearings, threshold)
		// First pair (60.002->60.004) is northward, should match.
		// Last event at 60.001 is node 1, but last match is at node ~4, so not advancing. Excluded.
		// Actually the last event is at node 1 which is < node 4, so it won't be added.
		// Wait - "lastNodeIndex > lastMatch.nodeIndex" means 1 > 4 is false, so excluded.
		if len(result) < 1 {
			t.Error("expected at least 1 match from valid pair")
		}
		// Check that last match nodeIndex is not 1 (the backward node)
		if len(result) > 0 {
			lastMatch := result[len(result)-1]
			if lastMatch.nodeIndex == 1 {
				t.Error("backward last event should not have been added")
			}
		}
	})

	// Verify hour is correctly extracted
	t.Run("hour extracted correctly", func(t *testing.T) {
		events := []types.VehicleEvent{
			{Longitude: 24.9, Latitude: 60.0, CreatedAt: 43200000},  // noon
			{Longitude: 24.9, Latitude: 60.004, CreatedAt: 46800000}, // 1pm
		}
		result := matchEventsToNodes(events, nodeIndex, shapeBearings, threshold)
		if len(result) > 0 && result[0].hour != 12 {
			t.Errorf("expected hour 12, got %d", result[0].hour)
		}
	})
}

func TestCalculateTravelTimeSamples(t *testing.T) {
	tests := []struct {
		name           string
		matches        []nodeEventMatch
		expectedCount  int
		checkSamples   func(t *testing.T, samples []nodeTravelTimeSample)
	}{
		{
			name:          "empty matches",
			matches:       []nodeEventMatch{},
			expectedCount: 0,
		},
		{
			name: "single match returns nil",
			matches: []nodeEventMatch{
				{nodeIndex: 0, createdAt: 1000, hour: 10},
			},
			expectedCount: 0,
		},
		{
			name: "two matches advancing",
			matches: []nodeEventMatch{
				{nodeIndex: 0, createdAt: 0, hour: 10},
				{nodeIndex: 2, createdAt: 2000, hour: 10},
			},
			expectedCount: 2, // nodes 1 and 2
			checkSamples: func(t *testing.T, samples []nodeTravelTimeSample) {
				for _, s := range samples {
					if math.Abs(s.travelTimeSeconds-1.0) > 0.001 {
						t.Errorf("expected 1.0s per node, got %f", s.travelTimeSeconds)
					}
					if s.hour != 10 {
						t.Errorf("expected hour 10, got %d", s.hour)
					}
				}
				if samples[0].nodeIndex != 1 || samples[1].nodeIndex != 2 {
					t.Errorf("expected nodes 1 and 2, got %d and %d", samples[0].nodeIndex, samples[1].nodeIndex)
				}
			},
		},
		{
			name: "two matches not advancing",
			matches: []nodeEventMatch{
				{nodeIndex: 2, createdAt: 0, hour: 10},
				{nodeIndex: 1, createdAt: 1000, hour: 10},
			},
			expectedCount: 0,
		},
		{
			name: "two matches same node",
			matches: []nodeEventMatch{
				{nodeIndex: 1, createdAt: 0, hour: 10},
				{nodeIndex: 1, createdAt: 1000, hour: 10},
			},
			expectedCount: 0,
		},
		{
			name: "three matches advancing",
			matches: []nodeEventMatch{
				{nodeIndex: 0, createdAt: 0, hour: 10},
				{nodeIndex: 3, createdAt: 3000, hour: 10},
				{nodeIndex: 5, createdAt: 5000, hour: 10},
			},
			expectedCount: 5, // nodes 1,2,3 from first pair + nodes 4,5 from second pair
			checkSamples: func(t *testing.T, samples []nodeTravelTimeSample) {
				for _, s := range samples {
					if math.Abs(s.travelTimeSeconds-1.0) > 0.001 {
						t.Errorf("expected 1.0s per node, got %f", s.travelTimeSeconds)
					}
				}
			},
		},
		{
			name: "large gap between nodes distributes time evenly",
			matches: []nodeEventMatch{
				{nodeIndex: 0, createdAt: 0, hour: 5},
				{nodeIndex: 10, createdAt: 10000, hour: 5},
			},
			expectedCount: 10,
			checkSamples: func(t *testing.T, samples []nodeTravelTimeSample) {
				for _, s := range samples {
					if math.Abs(s.travelTimeSeconds-1.0) > 0.001 {
						t.Errorf("expected 1.0s per node, got %f", s.travelTimeSeconds)
					}
				}
			},
		},
		{
			name: "mixed advancing and non-advancing",
			matches: []nodeEventMatch{
				{nodeIndex: 0, createdAt: 0, hour: 10},
				{nodeIndex: 3, createdAt: 3000, hour: 10},
				{nodeIndex: 2, createdAt: 4000, hour: 10}, // not advancing
				{nodeIndex: 5, createdAt: 7000, hour: 10},
			},
			expectedCount: 6, // nodes 1,2,3 from pair(0->3) + skip pair(3->2) + nodes 3,4,5 from pair(2->5)
			checkSamples: func(t *testing.T, samples []nodeTravelTimeSample) {
				// First 3 samples from pair 0->3: 1.0s each
				for i := 0; i < 3; i++ {
					if math.Abs(samples[i].travelTimeSeconds-1.0) > 0.001 {
						t.Errorf("sample %d: expected 1.0s, got %f", i, samples[i].travelTimeSeconds)
					}
				}
				// Last 3 samples from pair 2->5: 1.0s each (3000ms / 3 nodes)
				for i := 3; i < 6; i++ {
					if math.Abs(samples[i].travelTimeSeconds-1.0) > 0.001 {
						t.Errorf("sample %d: expected 1.0s, got %f", i, samples[i].travelTimeSeconds)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTravelTimeSamples(tt.matches)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d samples, got %d", tt.expectedCount, len(result))
			}
			if tt.checkSamples != nil && len(result) == tt.expectedCount {
				tt.checkSamples(t, result)
			}
		})
	}
}

func TestAggregateSamples(t *testing.T) {
	// Helper to create uint32 keys matching makeAggregationKey
	key := func(nodeIndex, hour int) uint32 {
		return uint32(nodeIndex)<<8 | uint32(hour&0xFF)
	}

	tests := []struct {
		name     string
		samples  []nodeTravelTimeSample
		expected map[uint32]struct {
			count int
			total float64
		}
	}{
		{
			name:     "empty samples",
			samples:  []nodeTravelTimeSample{},
			expected: map[uint32]struct{ count int; total float64 }{},
		},
		{
			name: "single sample",
			samples: []nodeTravelTimeSample{
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 5.0},
			},
			expected: map[uint32]struct{ count int; total float64 }{
				key(1, 10): {count: 1, total: 5.0},
			},
		},
		{
			name: "same node and hour aggregated",
			samples: []nodeTravelTimeSample{
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 5.0},
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 3.0},
			},
			expected: map[uint32]struct{ count int; total float64 }{
				key(1, 10): {count: 2, total: 8.0},
			},
		},
		{
			name: "same node different hours",
			samples: []nodeTravelTimeSample{
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 5.0},
				{nodeIndex: 1, hour: 11, travelTimeSeconds: 3.0},
			},
			expected: map[uint32]struct{ count int; total float64 }{
				key(1, 10): {count: 1, total: 5.0},
				key(1, 11): {count: 1, total: 3.0},
			},
		},
		{
			name: "different nodes same hour",
			samples: []nodeTravelTimeSample{
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 5.0},
				{nodeIndex: 2, hour: 10, travelTimeSeconds: 3.0},
			},
			expected: map[uint32]struct{ count int; total float64 }{
				key(1, 10): {count: 1, total: 5.0},
				key(2, 10): {count: 1, total: 3.0},
			},
		},
		{
			name: "multiple aggregations",
			samples: []nodeTravelTimeSample{
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 3.0},
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 6.0},
				{nodeIndex: 1, hour: 10, travelTimeSeconds: 9.0},
				{nodeIndex: 2, hour: 10, travelTimeSeconds: 4.0},
				{nodeIndex: 1, hour: 11, travelTimeSeconds: 2.0},
			},
			expected: map[uint32]struct{ count int; total float64 }{
				key(1, 10): {count: 3, total: 18.0},
				key(2, 10): {count: 1, total: 4.0},
				key(1, 11): {count: 1, total: 2.0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateSamples(tt.samples)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d groups, got %d", len(tt.expected), len(result))
			}

			for k, exp := range tt.expected {
				agg, exists := result[k]
				if !exists {
					t.Errorf("expected key %d not found", k)
					continue
				}
				if agg.sampleCount != exp.count {
					t.Errorf("key %d: expected count %d, got %d", k, exp.count, agg.sampleCount)
				}
				if math.Abs(agg.totalTravelTime-exp.total) > 0.001 {
					t.Errorf("key %d: expected total %f, got %f", k, exp.total, agg.totalTravelTime)
				}
			}
		})
	}
}
