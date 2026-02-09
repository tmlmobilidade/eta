package geo

import (
	"main/src/lib/geo"
	"main/src/types"
	"testing"
)

func TestNewNodeIndex(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []types.Coordinate
		precision uint
	}{
		{
			name:      "empty nodes",
			nodes:     []types.Coordinate{},
			precision: 7,
		},
		{
			name: "single node",
			nodes: []types.Coordinate{
				{-9.1393, 38.7223}, // Lisbon
			},
			precision: 7,
		},
		{
			name: "multiple nodes same geohash",
			nodes: []types.Coordinate{
				{-9.1393, 38.7223},
				{-9.1394, 38.7224},
				{-9.1395, 38.7225},
			},
			precision: 7,
		},
		{
			name: "nodes in different geohashes",
			nodes: []types.Coordinate{
				{-9.1393, 38.7223}, // Lisbon
				{-8.6291, 41.1579}, // Porto
			},
			precision: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := geo.NewNodeIndex(tt.nodes, tt.precision)
			if index == nil {
				t.Error("NewNodeIndex returned nil")
			}
		})
	}
}

func TestNodeIndex_FindNearest(t *testing.T) {
	// Create a line of nodes going north
	nodes := []types.Coordinate{
		{-9.14, 38.70}, // node 0
		{-9.14, 38.71}, // node 1
		{-9.14, 38.72}, // node 2
		{-9.14, 38.73}, // node 3
		{-9.14, 38.74}, // node 4
	}

	index := geo.NewNodeIndex(nodes, 7)

	tests := []struct {
		name          string
		lon           float64
		lat           float64
		expectedIndex int
	}{
		{
			name:          "exact match node 0",
			lon:           -9.14,
			lat:           38.70,
			expectedIndex: 0,
		},
		{
			name:          "exact match node 4",
			lon:           -9.14,
			lat:           38.74,
			expectedIndex: 4,
		},
		{
			name:          "closest to node 2",
			lon:           -9.14,
			lat:           38.7199,
			expectedIndex: 2,
		},
		{
			name:          "between nodes, closer to node 1",
			lon:           -9.14,
			lat:           38.712,
			expectedIndex: 1,
		},
		{
			name:          "between nodes, closer to node 2",
			lon:           -9.14,
			lat:           38.718,
			expectedIndex: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := index.FindNearest(tt.lon, tt.lat)
			if result != tt.expectedIndex {
				t.Errorf("FindNearest(%f, %f) = %d, want %d", tt.lon, tt.lat, result, tt.expectedIndex)
			}
		})
	}
}

func TestNodeIndex_FindNearest_Empty(t *testing.T) {
	index := geo.NewNodeIndex([]types.Coordinate{}, 7)
	result := index.FindNearest(-9.14, 38.72)
	if result != 0 {
		t.Errorf("FindNearest on empty index should return 0, got %d", result)
	}
}

func TestNodeIndex_FindNearest_SingleNode(t *testing.T) {
	nodes := []types.Coordinate{
		{-9.14, 38.72},
	}
	index := geo.NewNodeIndex(nodes, 7)

	// Query at the exact node location
	result := index.FindNearest(-9.14, 38.72)
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}

	// Query far from the node - should still return 0 (only node)
	result = index.FindNearest(-8.0, 40.0)
	if result != 0 {
		t.Errorf("expected 0 for distant point, got %d", result)
	}
}

// Benchmark to verify geohash indexing is faster than linear search
func BenchmarkFindNearest_Linear(b *testing.B) {
	// Create 500 nodes (typical shape size)
	nodes := make([]types.Coordinate, 500)
	for i := range 500 {
		nodes[i] = types.Coordinate{-9.14, 38.70 + float64(i)*0.0001}
	}

	queryLon, queryLat := -9.14, 38.72

	b.ResetTimer()
	for range b.N {
		geo.FindNearestNodeIndex(queryLon, queryLat, nodes)
	}
}

func BenchmarkFindNearest_NodeIndex(b *testing.B) {
	// Create 500 nodes (typical shape size)
	nodes := make([]types.Coordinate, 500)
	for i := range 500 {
		nodes[i] = types.Coordinate{-9.14, 38.70 + float64(i)*0.0001}
	}

	index := geo.NewNodeIndex(nodes, 7)
	queryLon, queryLat := -9.14, 38.72

	b.ResetTimer()
	for range b.N {
		index.FindNearest(queryLon, queryLat)
	}
}
