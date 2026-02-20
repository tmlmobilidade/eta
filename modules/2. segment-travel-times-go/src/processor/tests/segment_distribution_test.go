package processor_test

import (
	"main/src/processor"
	"main/src/types"
	"testing"
)

func TestComputeSegmentMetrics(t *testing.T) {
	type input struct {
		prevEvent   types.VehicleEvent
		currEvent   types.VehicleEvent
		prevNodeIdx int
		currNodeIdx int
	}

	type expected struct {
		nodeDelta   int
		timeDelta   float64
		distance    float64
		speedKmh    float64
		timePerNode float64
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "4 nodes in 8 seconds",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_008_000},
				prevNodeIdx: 0,
				currNodeIdx: 4,
			},
			expected: expected{
				nodeDelta:   4,
				timeDelta:   8.0,
				distance:    100.0, // 4 × 25m
				speedKmh:    45.0,  // (100 / 8) × 3.6
				timePerNode: 2.0,   // 8 / 4
			},
		},
		{
			name: "single node in 5 seconds",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 0},
				currEvent:   types.VehicleEvent{CreatedAt: 5_000},
				prevNodeIdx: 3,
				currNodeIdx: 4,
			},
			expected: expected{
				nodeDelta:   1,
				timeDelta:   5.0,
				distance:    25.0,
				speedKmh:    18.0, // (25 / 5) × 3.6
				timePerNode: 5.0,
			},
		},
		{
			name: "backward movement yields negative delta",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 0},
				currEvent:   types.VehicleEvent{CreatedAt: 5_000},
				prevNodeIdx: 4,
				currNodeIdx: 2,
			},
			expected: expected{
				nodeDelta:   -2,
				timeDelta:   5.0,
				distance:    -50.0,
				speedKmh:    -36.0,
				timePerNode: -2.5,
			},
		},
		{
			name: "10 nodes in 20 seconds",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 500_000},
				currEvent:   types.VehicleEvent{CreatedAt: 520_000},
				prevNodeIdx: 0,
				currNodeIdx: 10,
			},
			expected: expected{
				nodeDelta:   10,
				timeDelta:   20.0,
				distance:    250.0, // 10 × 25m
				speedKmh:    45.0,  // (250 / 20) × 3.6
				timePerNode: 2.0,   // 20 / 10
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeDelta, timeDelta, distance, speedKmh, timePerNode := processor.ComputeSegmentMetrics(
				&tt.input.prevEvent, &tt.input.currEvent,
				tt.input.prevNodeIdx, tt.input.currNodeIdx,
			)

			if nodeDelta != tt.expected.nodeDelta {
				t.Errorf("nodeDelta = %d, want %d", nodeDelta, tt.expected.nodeDelta)
			}
			if timeDelta != tt.expected.timeDelta {
				t.Errorf("timeDelta = %f, want %f", timeDelta, tt.expected.timeDelta)
			}
			if distance != tt.expected.distance {
				t.Errorf("distance = %f, want %f", distance, tt.expected.distance)
			}
			if speedKmh != tt.expected.speedKmh {
				t.Errorf("speedKmh = %f, want %f", speedKmh, tt.expected.speedKmh)
			}
			if timePerNode != tt.expected.timePerNode {
				t.Errorf("timePerNode = %f, want %f", timePerNode, tt.expected.timePerNode)
			}
		})
	}
}

func TestDistributeSegmentTravelTime(t *testing.T) {
	type input struct {
		prevEvent   types.VehicleEvent
		currEvent   types.VehicleEvent
		prevNodeIdx int
		currNodeIdx int
		shapeID     string
	}

	type expected struct {
		totalSamples  int
		sampleValue   float64
		affectedNodes int
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "valid segment distributes to all intermediate nodes",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_008_000}, // 8s, 45 km/h
				prevNodeIdx: 0,
				currNodeIdx: 4,
				shapeID:     "shape-1",
			},
			expected: expected{
				totalSamples:  4,   // nodes 0, 1, 2, 3
				sampleValue:   2.0, // 8s / 4 nodes
				affectedNodes: 4,
			},
		},
		{
			name: "backward direction is discarded",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_008_000},
				prevNodeIdx: 4,
				currNodeIdx: 2,
				shapeID:     "shape-1",
			},
			expected: expected{totalSamples: 0},
		},
		{
			name: "same node index is discarded",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_005_000},
				prevNodeIdx: 3,
				currNodeIdx: 3,
				shapeID:     "shape-1",
			},
			expected: expected{totalSamples: 0},
		},
		{
			name: "zero time delta is discarded",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				prevNodeIdx: 0,
				currNodeIdx: 4,
				shapeID:     "shape-1",
			},
			expected: expected{totalSamples: 0},
		},
		{
			name: "speed above 120 km/h is discarded",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_001_000}, // 1s → 180 km/h
				prevNodeIdx: 0,
				currNodeIdx: 2,
				shapeID:     "shape-1",
			},
			expected: expected{totalSamples: 0},
		},
		{
			name: "speed below 1 km/h is discarded",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_100_000}, // 100s → 0.9 km/h
				prevNodeIdx: 0,
				currNodeIdx: 1,
				shapeID:     "shape-1",
			},
			expected: expected{totalSamples: 0},
		},
		{
			name: "single node jump at valid speed",
			input: input{
				prevEvent:   types.VehicleEvent{CreatedAt: 1_000_000},
				currEvent:   types.VehicleEvent{CreatedAt: 1_005_000}, // 5s → 18 km/h
				prevNodeIdx: 0,
				currNodeIdx: 1,
				shapeID:     "shape-1",
			},
			expected: expected{
				totalSamples:  1,
				sampleValue:   5.0,
				affectedNodes: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accumulators := make(map[string]map[types.NodeHourKey]*types.NodeAccumulator)

			processor.DistributeSegmentTravelTime(
				&tt.input.prevEvent, &tt.input.currEvent,
				tt.input.prevNodeIdx, tt.input.currNodeIdx,
				tt.input.shapeID, accumulators,
			)

			totalSamples := 0
			nodeCount := 0
			for _, nodeAccs := range accumulators {
				nodeCount += len(nodeAccs)
				for _, acc := range nodeAccs {
					for _, sample := range acc.Samples {
						totalSamples++
						if tt.expected.sampleValue > 0 && sample != tt.expected.sampleValue {
							t.Errorf("sample = %f, want %f", sample, tt.expected.sampleValue)
						}
					}
				}
			}

			if totalSamples != tt.expected.totalSamples {
				t.Errorf("totalSamples = %d, want %d", totalSamples, tt.expected.totalSamples)
			}
			if tt.expected.affectedNodes > 0 && nodeCount != tt.expected.affectedNodes {
				t.Errorf("affectedNodes = %d, want %d", nodeCount, tt.expected.affectedNodes)
			}
		})
	}
}

func TestDistributeSegmentTravelTime_AccumulatesSamples(t *testing.T) {
	accumulators := make(map[string]map[types.NodeHourKey]*types.NodeAccumulator)

	// Two segments that overlap on nodes 2 and 3.
	// Segment 1: nodes 0→4, 8 seconds at 45 km/h → timePerNode = 2.0
	seg1Prev := types.VehicleEvent{CreatedAt: 1_000_000}
	seg1Curr := types.VehicleEvent{CreatedAt: 1_008_000}
	processor.DistributeSegmentTravelTime(&seg1Prev, &seg1Curr, 0, 4, "shape-1", accumulators)

	// Segment 2: nodes 2→6, 10 seconds at 36 km/h → timePerNode = 2.5
	seg2Prev := types.VehicleEvent{CreatedAt: 2_000_000}
	seg2Curr := types.VehicleEvent{CreatedAt: 2_010_000}
	processor.DistributeSegmentTravelTime(&seg2Prev, &seg2Curr, 2, 6, "shape-1", accumulators)

	totalSamples := 0
	for _, nodeAccs := range accumulators {
		for _, acc := range nodeAccs {
			totalSamples += len(acc.Samples)
		}
	}

	// Segment 1 contributes 4 samples (nodes 0,1,2,3).
	// Segment 2 contributes 4 samples (nodes 2,3,4,5).
	// Total = 8 samples across all node-hour keys.
	wantTotal := 8
	if totalSamples != wantTotal {
		t.Errorf("totalSamples = %d, want %d", totalSamples, wantTotal)
	}
}
