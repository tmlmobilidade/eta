package processor

import (
	"context"
	"errors"
	"fmt"
	"main/src/types"
	"testing"
)

// mockClickhouse implements ClickhouseClient for testing.
type mockClickhouse struct {
	fetchResult  []types.VehicleEvent
	fetchErr     error
	deleteErr    error
	saveErr      error
	savedRecords []types.NodeTravelTimeRecord
}

func (m *mockClickhouse) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error) {
	return m.fetchResult, m.fetchErr
}

func (m *mockClickhouse) FetchVehicleEventsByGeohash(ctx context.Context, geohashes []string, settings *types.Settings) (map[string][]types.VehicleEvent, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	// Group events by geohash (for mock, just put all events in each requested geohash)
	result := make(map[string][]types.VehicleEvent)
	for _, gh := range geohashes {
		result[gh] = m.fetchResult
	}
	return result, nil
}

func (m *mockClickhouse) DeleteTravelTimesForShapes(ctx context.Context, lineID uint32, hashedShapeIDs []string) error {
	return m.deleteErr
}

func (m *mockClickhouse) SaveTravelTimes(ctx context.Context, records []types.NodeTravelTimeRecord) error {
	m.savedRecords = append(m.savedRecords, records...)
	return m.saveErr
}

func TestFetchAndPrepareVehicleEvents(t *testing.T) {
	ctx := context.Background()

	lineData := &types.LineShapeData{
		Geohashes:      map[string]struct{}{"u4x": {}},
		HashedShapeIDs: []string{"shape-1"},
		Nodes:          map[string][]types.Coordinate{},
	}

	tests := []struct {
		name        string
		mock        *mockClickhouse
		expectNil   bool
		expectErr   bool
	}{
		{
			name: "successful fetch",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{
					{TripOperationalID: "t1", CreatedAt: 1000},
					{TripOperationalID: "t1", CreatedAt: 2000},
				},
			},
			expectNil: false,
			expectErr: false,
		},
		{
			name: "fetch returns error",
			mock: &mockClickhouse{
				fetchErr: errors.New("connection failed"),
			},
			expectNil: true,
			expectErr: true,
		},
		{
			name: "empty events returns nil nil",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{},
			},
			expectNil: true,
			expectErr: false,
		},
		{
			name: "delete fails after successful fetch",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{
					{TripOperationalID: "t1", CreatedAt: 1000},
				},
				deleteErr: errors.New("delete failed"),
			},
			expectNil: true,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := NewLineProcessor(tt.mock, &types.Settings{})
			result, err := lp.fetchAndPrepareVehicleEvents(ctx, 1, lineData)

			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectNil && result != nil {
				t.Errorf("expected nil result, got %d events", len(result))
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result, got nil")
			}
		})
	}
}

func TestProcessAllLines(t *testing.T) {
	ctx := context.Background()

	shapeNodes := []types.Coordinate{
		{24.9, 60.0},
		{24.9, 60.001},
		{24.9, 60.002},
		{24.9, 60.003},
		{24.9, 60.004},
	}

	tests := []struct {
		name      string
		mock      *mockClickhouse
		lineMap   types.LineShapesMap
		expectErr bool
	}{
		{
			name:      "empty line map",
			mock:      &mockClickhouse{},
			lineMap:   types.LineShapesMap{},
			expectErr: false,
		},
		{
			name: "single line no events skipped",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{},
			},
			lineMap: types.LineShapesMap{
				1: &types.LineShapeData{
					Geohashes:      map[string]struct{}{"u4x": {}},
					HashedShapeIDs: []string{"shape-1"},
					Nodes: map[string][]types.Coordinate{
						"shape-1": shapeNodes,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "single line successful processing",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{
					{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
					{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
				},
			},
			lineMap: types.LineShapesMap{
				1: &types.LineShapeData{
					Geohashes:      map[string]struct{}{"u4x": {}},
					HashedShapeIDs: []string{"shape-1"},
					Nodes: map[string][]types.Coordinate{
						"shape-1": shapeNodes,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "fetch error propagated",
			mock: &mockClickhouse{
				fetchErr: errors.New("connection failed"),
			},
			lineMap: types.LineShapesMap{
				1: &types.LineShapeData{
					Geohashes:      map[string]struct{}{"u4x": {}},
					HashedShapeIDs: []string{"shape-1"},
					Nodes:          map[string][]types.Coordinate{},
				},
			},
			expectErr: true,
		},
		{
			name: "save error propagated",
			mock: &mockClickhouse{
				fetchResult: []types.VehicleEvent{
					{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
					{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
				},
				saveErr: errors.New("save failed"),
			},
			lineMap: types.LineShapesMap{
				1: &types.LineShapeData{
					Geohashes:      map[string]struct{}{"u4x": {}},
					HashedShapeIDs: []string{"shape-1"},
					Nodes: map[string][]types.Coordinate{
						"shape-1": shapeNodes,
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := NewLineProcessor(tt.mock, &types.Settings{
				BearingThreshold: 45,
				WorkerCount:      1,
				BatchSize:        10,
			})
			err := lp.ProcessAllLines(ctx, tt.lineMap)

			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	// Verify records are saved via mock
	t.Run("records saved to clickhouse", func(t *testing.T) {
		mock := &mockClickhouse{
			fetchResult: []types.VehicleEvent{
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
				{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
			},
		}
		lp := NewLineProcessor(mock, &types.Settings{
			BearingThreshold: 45,
			WorkerCount:      1,
			BatchSize:        10,
		})
		lineMap := types.LineShapesMap{
			1: &types.LineShapeData{
				Geohashes:      map[string]struct{}{"u4x": {}},
				HashedShapeIDs: []string{"shape-1"},
				Nodes: map[string][]types.Coordinate{
					"shape-1": shapeNodes,
				},
			},
		}
		err := lp.ProcessAllLines(ctx, lineMap)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mock.savedRecords) == 0 {
			t.Error("expected records to be saved, but none were")
		}
	})
}

func TestProcessAllLines_MultipleBatches(t *testing.T) {
	ctx := context.Background()

	shapeNodes := []types.Coordinate{
		{24.9, 60.0},
		{24.9, 60.001},
		{24.9, 60.002},
		{24.9, 60.003},
		{24.9, 60.004},
	}

	mock := &mockClickhouse{
		fetchResult: []types.VehicleEvent{
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
		},
	}

	// Create 5 lines with batch size 2 -> 3 batches (2, 2, 1)
	lineMap := types.LineShapesMap{}
	for i := 1; i <= 5; i++ {
		lineMap[i] = &types.LineShapeData{
			Geohashes:      map[string]struct{}{"u4x": {}},
			HashedShapeIDs: []string{fmt.Sprintf("shape-%d", i)},
			Nodes:          map[string][]types.Coordinate{fmt.Sprintf("shape-%d", i): shapeNodes},
		}
	}

	lp := NewLineProcessor(mock, &types.Settings{
		BearingThreshold: 45,
		WorkerCount:      2,
		BatchSize:        2,
	})

	err := lp.ProcessAllLines(ctx, lineMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.savedRecords) == 0 {
		t.Error("expected records from all 5 lines, got none")
	}
}

func TestProcessAllLines_BatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	shapeNodes := []types.Coordinate{
		{24.9, 60.0},
		{24.9, 60.001},
		{24.9, 60.002},
		{24.9, 60.003},
		{24.9, 60.004},
	}

	mock := &mockClickhouse{
		fetchResult: []types.VehicleEvent{
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.0, CreatedAt: 0},
			{TripOperationalID: "t1", Longitude: 24.9, Latitude: 60.004, CreatedAt: 4000},
		},
	}

	// Create 4 lines with batch size 1 -> 4 batches
	lineMap := types.LineShapesMap{}
	for i := 1; i <= 4; i++ {
		lineMap[i] = &types.LineShapeData{
			Geohashes:      map[string]struct{}{"u4x": {}},
			HashedShapeIDs: []string{fmt.Sprintf("shape-%d", i)},
			Nodes:          map[string][]types.Coordinate{fmt.Sprintf("shape-%d", i): shapeNodes},
		}
	}

	// Cancel immediately so the context is already done before processing starts
	cancel()

	lp := NewLineProcessor(mock, &types.Settings{
		BearingThreshold: 45,
		WorkerCount:      1,
		BatchSize:        1,
	})

	err := lp.ProcessAllLines(ctx, lineMap)
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
