package processor_test

import (
	"main/src/processor"
	"main/src/types"
	"testing"
)

func TestBuildRecords(t *testing.T) {
	type input struct {
		accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator
		shapeNodes   map[string][]types.Coordinate
	}

	tests := []struct {
		name     string
		input    input
		expected []types.NodeTravelTimeRecord
	}{
		{
			name: "empty accumulators produces no records",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{},
				shapeNodes:   map[string][]types.Coordinate{},
			},
			expected: nil,
		},
		{
			name: "single accumulator produces one record",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 0, Hour: 10}: {Samples: []float64{2.0, 4.0, 6.0}},
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}, {-9.15, 38.71}},
				},
			},
			expected: []types.NodeTravelTimeRecord{
				{
					ShapeID:           "shape-1",
					NodeIndex:         0,
					Hour:              10,
					Latitude:          38.70,
					Longitude:         -9.15,
					TravelTimeSeconds: 4.0, // median of [2, 4, 6]
					SampleCount:       3,
				},
			},
		},
		{
			name: "even number of samples uses average of middle two",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 0, Hour: 8}: {Samples: []float64{1.0, 2.0, 3.0, 4.0}},
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}},
				},
			},
			expected: []types.NodeTravelTimeRecord{
				{
					ShapeID:           "shape-1",
					NodeIndex:         0,
					Hour:              8,
					Latitude:          38.70,
					Longitude:         -9.15,
					TravelTimeSeconds: 2.5, // median of [1, 2, 3, 4] = (2+3)/2
					SampleCount:       4,
				},
			},
		},
		{
			name: "node index out of bounds is skipped",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 5, Hour: 10}: {Samples: []float64{2.0}},
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}, {-9.15, 38.71}},
				},
			},
			expected: nil,
		},
		{
			name: "empty samples are skipped",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 0, Hour: 10}: {Samples: []float64{}},
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}},
				},
			},
			expected: nil,
		},
		{
			name: "multiple shapes and hours produce separate records",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 0, Hour: 8}:  {Samples: []float64{3.0}},
						{NodeIdx: 0, Hour: 18}: {Samples: []float64{5.0}},
					},
					"shape-2": {
						{NodeIdx: 1, Hour: 12}: {Samples: []float64{1.0, 2.0}},
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}, {-9.15, 38.71}},
					"shape-2": {{-8.60, 41.15}, {-8.60, 41.16}},
				},
			},
			expected: []types.NodeTravelTimeRecord{
				{ShapeID: "shape-1", NodeIndex: 0, Hour: 8, Latitude: 38.70, Longitude: -9.15, TravelTimeSeconds: 3.0, SampleCount: 1},
				{ShapeID: "shape-1", NodeIndex: 0, Hour: 18, Latitude: 38.70, Longitude: -9.15, TravelTimeSeconds: 5.0, SampleCount: 1},
				{ShapeID: "shape-2", NodeIndex: 1, Hour: 12, Latitude: 41.16, Longitude: -8.60, TravelTimeSeconds: 1.5, SampleCount: 2},
			},
		},
		{
			name: "mixed valid and invalid entries",
			input: input{
				accumulators: map[string]map[types.NodeHourKey]*types.NodeAccumulator{
					"shape-1": {
						{NodeIdx: 0, Hour: 10}: {Samples: []float64{2.0}},  // valid
						{NodeIdx: 9, Hour: 10}: {Samples: []float64{3.0}},  // out of bounds
						{NodeIdx: 1, Hour: 12}: {Samples: []float64{}},     // empty samples
					},
				},
				shapeNodes: map[string][]types.Coordinate{
					"shape-1": {{-9.15, 38.70}, {-9.15, 38.71}},
				},
			},
			expected: []types.NodeTravelTimeRecord{
				{ShapeID: "shape-1", NodeIndex: 0, Hour: 10, Latitude: 38.70, Longitude: -9.15, TravelTimeSeconds: 2.0, SampleCount: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := processor.BuildRecords(tt.input.accumulators, tt.input.shapeNodes)

			if tt.expected == nil {
				if len(records) != 0 {
					t.Errorf("got %d records, want 0", len(records))
				}
				return
			}

			if len(records) != len(tt.expected) {
				t.Fatalf("got %d records, want %d", len(records), len(tt.expected))
			}

			type recordKey struct {
				ShapeID   string
				NodeIndex int
				Hour      uint8
			}

			lookup := make(map[recordKey]types.NodeTravelTimeRecord)
			for _, r := range tt.expected {
				lookup[recordKey{r.ShapeID, r.NodeIndex, r.Hour}] = r
			}

			for _, got := range records {
				key := recordKey{got.ShapeID, got.NodeIndex, got.Hour}
				want, exists := lookup[key]
				if !exists {
					t.Errorf("unexpected record: %+v", got)
					continue
				}
				if got != want {
					t.Errorf("record mismatch for key %+v:\n  got  %+v\n  want %+v", key, got, want)
				}
			}
		})
	}
}
