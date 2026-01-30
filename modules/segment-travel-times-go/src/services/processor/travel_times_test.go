package processor

import (
	"main/src/types"
	"testing"
)

func TestProcessShapeTravelTimes(t *testing.T) {
	lp := &LineProcessor{
		clickhouse: nil,
		settings:   &types.Settings{BearingThreshold: 45},
	}

	// Simple north-south shape (nodes go south to north)
	nodes := []types.Coordinate{
		{24.9, 60.0},
		{24.9, 60.001},
		{24.9, 60.002},
		{24.9, 60.003},
		{24.9, 60.004},
	}

	tests := []struct {
		name            string
		events          []types.VehicleEvent
		nodes           []types.Coordinate
		lineID          int
		hashedShapeID   string
		expectEmpty     bool
		minRecordCount  int
	}{
		{
			name:        "empty events",
			events:      []types.VehicleEvent{},
			nodes:       nodes,
			lineID:      1,
			expectEmpty: true,
		},
		{
			name: "fewer than 2 nodes",
			events: []types.VehicleEvent{
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 1000},
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.001, CreatedAt: 2000},
			},
			nodes:       []types.Coordinate{{24.9, 60.0}},
			lineID:      1,
			expectEmpty: true,
		},
		{
			name: "single trip with two events along shape",
			events: []types.VehicleEvent{
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
			},
			nodes:          nodes,
			lineID:         42,
			hashedShapeID:  "shape-abc",
			expectEmpty:    false,
			minRecordCount: 1,
		},
		{
			name: "trip with single event skipped",
			events: []types.VehicleEvent{
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
			},
			nodes:       nodes,
			lineID:      1,
			expectEmpty: true,
		},
		{
			name: "all events wrong bearing",
			events: []types.VehicleEvent{
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 0},
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 4000},
			},
			nodes:       nodes,
			lineID:      1,
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lp.processShapeTravelTimes(tt.events, tt.nodes, tt.lineID, tt.hashedShapeID)
			if tt.expectEmpty {
				if len(result) != 0 {
					t.Errorf("expected empty result, got %d records", len(result))
				}
				return
			}
			if len(result) < tt.minRecordCount {
				t.Errorf("expected at least %d records, got %d", tt.minRecordCount, len(result))
			}
		})
	}

	// Verify record fields
	t.Run("verify record fields", func(t *testing.T) {
		events := []types.VehicleEvent{
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
		}
		result := lp.processShapeTravelTimes(events, nodes, 42, "shape-xyz")
		if len(result) == 0 {
			t.Fatal("expected records, got none")
		}
		for _, r := range result {
			if r.LineID != 42 {
				t.Errorf("expected LineID 42, got %d", r.LineID)
			}
			if r.HashedShapeID != "shape-xyz" {
				t.Errorf("expected HashedShapeID 'shape-xyz', got %s", r.HashedShapeID)
			}
			if r.SampleCount == 0 {
				t.Error("expected non-zero SampleCount")
			}
			if r.TravelTimeSeconds <= 0 {
				t.Errorf("expected positive TravelTimeSeconds, got %f", r.TravelTimeSeconds)
			}
		}
	})

	// Verify multiple trips are aggregated
	t.Run("multiple trips aggregated", func(t *testing.T) {
		events := []types.VehicleEvent{
			// Trip 1
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
			// Trip 2
			{TripOperationalID: "t2", Longitude: 24.9, Latitude: 60.0, CreatedAt: 10000},
			{TripOperationalID: "t2", Longitude: 24.9, Latitude: 60.004, CreatedAt: 14000},
		}
		result := lp.processShapeTravelTimes(events, nodes, 1, "shape-1")
		if len(result) == 0 {
			t.Fatal("expected records from multiple trips")
		}
		// Check that sample counts reflect multiple trips
		totalSamples := uint32(0)
		for _, r := range result {
			totalSamples += r.SampleCount
		}
		if totalSamples < 2 {
			t.Errorf("expected at least 2 total samples from 2 trips, got %d", totalSamples)
		}
	})
}

func TestProcessLine(t *testing.T) {
	lp := &LineProcessor{
		clickhouse: nil,
		settings:   &types.Settings{BearingThreshold: 45},
	}

	// Simple north-south shape nodes
	shapeNodes := []types.Coordinate{
		{24.9, 60.0},
		{24.9, 60.001},
		{24.9, 60.002},
		{24.9, 60.003},
		{24.9, 60.004},
	}

	northboundEvents := []types.VehicleEvent{
		{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
		{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
	}

	tests := []struct {
		name        string
		events      []types.VehicleEvent
		lineID      int
		lineData    *types.LineShapeData
		expectEmpty bool
	}{
		{
			name:   "no shapes",
			events: northboundEvents,
			lineID: 1,
			lineData: &types.LineShapeData{
				HashedShapeIDs: []string{},
				Nodes:          map[string][]types.Coordinate{},
				Geohashes:      map[string]struct{}{},
			},
			expectEmpty: true,
		},
		{
			name:   "shape not in nodes map",
			events: northboundEvents,
			lineID: 1,
			lineData: &types.LineShapeData{
				HashedShapeIDs: []string{"missing-shape"},
				Nodes:          map[string][]types.Coordinate{},
				Geohashes:      map[string]struct{}{},
			},
			expectEmpty: true,
		},
		{
			name:   "shape with fewer than 2 nodes",
			events: northboundEvents,
			lineID: 1,
			lineData: &types.LineShapeData{
				HashedShapeIDs: []string{"shape-1"},
				Nodes: map[string][]types.Coordinate{
					"shape-1": {{24.9, 60.0}}, // only 1 node
				},
				Geohashes: map[string]struct{}{},
			},
			expectEmpty: true,
		},
		{
			name:   "one shape with valid data",
			events: northboundEvents,
			lineID: 1,
			lineData: &types.LineShapeData{
				HashedShapeIDs: []string{"shape-1"},
				Nodes: map[string][]types.Coordinate{
					"shape-1": shapeNodes,
				},
				Geohashes: map[string]struct{}{},
			},
			expectEmpty: false,
		},
		{
			name:   "multiple shapes",
			events: northboundEvents,
			lineID: 1,
			lineData: &types.LineShapeData{
				HashedShapeIDs: []string{"shape-1", "shape-2"},
				Nodes: map[string][]types.Coordinate{
					"shape-1": shapeNodes,
					"shape-2": shapeNodes,
				},
				Geohashes: map[string]struct{}{},
			},
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lp.processLine(tt.events, tt.lineID, tt.lineData)
			if tt.expectEmpty && len(result) != 0 {
				t.Errorf("expected empty result, got %d records", len(result))
			}
			if !tt.expectEmpty && len(result) == 0 {
				t.Error("expected records, got none")
			}
		})
	}
}
