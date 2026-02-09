package geo

import (
	"main/src/types"
	"math"

	"github.com/mmcloughlin/geohash"
)

// indexedNode stores a node with its original index for lookup.
type indexedNode struct {
	index int
	coord types.Coordinate
}

// NodeIndex provides O(1) average-case nearest-node lookups using geohash bucketing.
// Instead of scanning all nodes for each event, it only searches nodes in nearby geohash cells.
type NodeIndex struct {
	buckets   map[string][]indexedNode // geohash -> nodes in that cell
	nodes     []types.Coordinate       // original nodes for fallback
	precision uint                     // geohash precision for bucketing
}

// NewNodeIndex creates a NodeIndex from a slice of coordinates.
// Precision 7 (~153m x 153m cells) is recommended for 50m segments.
func NewNodeIndex(nodes []types.Coordinate, precision uint) *NodeIndex {
	if precision == 0 {
		precision = 7 // default precision
	}

	ni := &NodeIndex{
		buckets:   make(map[string][]indexedNode),
		nodes:     nodes,
		precision: precision,
	}

	// Bucket all nodes by their geohash
	for i, node := range nodes {
		hash := geohash.EncodeWithPrecision(node.Latitude(), node.Longitude(), precision)
		ni.buckets[hash] = append(ni.buckets[hash], indexedNode{
			index: i,
			coord: node,
		})
	}

	return ni
}

// FindNearest finds the index of the nearest node to the given coordinates.
// Uses geohash bucketing for O(1) average case instead of O(n) linear scan.
func (ni *NodeIndex) FindNearest(lon, lat float64) int {
	if len(ni.nodes) == 0 {
		return 0
	}

	// Get the geohash for the query point
	queryHash := geohash.EncodeWithPrecision(lat, lon, ni.precision)

	// Get candidates from this cell and all 8 neighbors
	candidates := ni.getCandidates(queryHash)

	// If no candidates in nearby cells, fall back to linear search
	// This can happen at geohash boundaries or with sparse data
	if len(candidates) == 0 {
		return findNearestLinear(lon, lat, ni.nodes)
	}

	// Find nearest among candidates
	queryCoord := types.Coordinate{lon, lat}
	nearestIndex := candidates[0].index
	minDistance := HaversineDistance(queryCoord, candidates[0].coord)

	for _, candidate := range candidates[1:] {
		distance := HaversineDistance(queryCoord, candidate.coord)
		if distance < minDistance {
			minDistance = distance
			nearestIndex = candidate.index
		}
	}

	return nearestIndex
}

// getCandidates returns all nodes in the given geohash cell and its 8 neighbors.
func (ni *NodeIndex) getCandidates(centerHash string) []indexedNode {
	// Get neighbors (returns 8 surrounding cells)
	neighbors := geohash.Neighbors(centerHash)

	// Estimate capacity: center + 8 neighbors, ~5 nodes per cell average
	candidates := make([]indexedNode, 0, 45)

	// Add nodes from center cell
	if nodes, ok := ni.buckets[centerHash]; ok {
		candidates = append(candidates, nodes...)
	}

	// Add nodes from all 8 neighboring cells
	for _, neighborHash := range neighbors {
		if nodes, ok := ni.buckets[neighborHash]; ok {
			candidates = append(candidates, nodes...)
		}
	}

	return candidates
}

// findNearestLinear is the fallback O(n) search when geohash lookup fails.
func findNearestLinear(eventLon, eventLat float64, nodes []types.Coordinate) int {
	eventCoord := types.Coordinate{eventLon, eventLat}
	nearestIndex := 0
	minDistance := math.MaxFloat64

	for i, node := range nodes {
		distance := HaversineDistance(eventCoord, node)
		if distance < minDistance {
			minDistance = distance
			nearestIndex = i
		}
	}

	return nearestIndex
}
