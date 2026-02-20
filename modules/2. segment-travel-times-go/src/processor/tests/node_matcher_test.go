package processor_test

import (
	"main/src/processor"
	"main/src/types"
	"testing"
)

func TestMatchEventToNode(t *testing.T) {
	nodes := []types.Coordinate{
		{-9.15, 38.70},
		{-9.15, 38.71},
		{-9.15, 38.72},
		{-9.15, 38.73},
	}

	type input struct {
		event       types.VehicleEvent
		nodes       []types.Coordinate
		maxDistance float64
	}

	type expected struct {
		node    types.Coordinate
		matched bool
	}

	tests := []struct {
		name     string
		input    input
		expected expected
	}{
		{
			name: "exact match on first node",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.15, Latitude: 38.70},
				nodes:       nodes,
				maxDistance: 100,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.70},
				matched: true,
			},
		},
		{
			name: "closest to second node",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.15, Latitude: 38.709},
				nodes:       nodes,
				maxDistance: 500,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.71},
				matched: true,
			},
		},
		{
			name: "closest to last node",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.15, Latitude: 38.728},
				nodes:       nodes,
				maxDistance: 500,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.73},
				matched: true,
			},
		},
		{
			name: "event too far from any node",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.10, Latitude: 38.70},
				nodes:       nodes,
				maxDistance: 100,
			},
			expected: expected{
				node:    types.Coordinate{},
				matched: false,
			},
		},
		{
			name: "no distance limit when maxDistance is zero",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.10, Latitude: 38.70},
				nodes:       nodes,
				maxDistance: 0,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.70},
				matched: true,
			},
		},
		{
			name: "no distance limit when maxDistance is negative",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.10, Latitude: 38.70},
				nodes:       nodes,
				maxDistance: -1,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.70},
				matched: true,
			},
		},
		{
			name: "single node within distance",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.15, Latitude: 38.70},
				nodes:       []types.Coordinate{{-9.15, 38.70}},
				maxDistance: 100,
			},
			expected: expected{
				node:    types.Coordinate{-9.15, 38.70},
				matched: true,
			},
		},
		{
			name: "single node beyond distance",
			input: input{
				event:       types.VehicleEvent{Longitude: -9.10, Latitude: 38.70},
				nodes:       []types.Coordinate{{-9.15, 38.70}},
				maxDistance: 100,
			},
			expected: expected{
				node:    types.Coordinate{},
				matched: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, matched := processor.MatchEventToNode(tt.input.event, tt.input.nodes, tt.input.maxDistance)

			if matched != tt.expected.matched {
				t.Errorf("matched = %v, want %v", matched, tt.expected.matched)
			}
			if matched && node != tt.expected.node {
				t.Errorf("node = %v, want %v", node, tt.expected.node)
			}
		})
	}
}
