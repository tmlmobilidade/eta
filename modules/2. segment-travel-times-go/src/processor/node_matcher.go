package processor

import (
	"main/src/lib/geo"
	"main/src/types"
)

// MatchEventToNode finds the nearest node to the event and enforces an optional maximum distance limit (in meters).
func MatchEventToNode(event types.VehicleEvent, nodes []types.Coordinate, maxNodeMatchDistanceMeters float64) (types.Coordinate, bool) {
	eventCoord := types.Coordinate{event.Longitude, event.Latitude}

	nearestNode := nodes[0]
	nearestDistance := geo.HaversineDistance(eventCoord, nearestNode)

	for i := 1; i < len(nodes); i++ {
		node := nodes[i]
		distance := geo.HaversineDistance(eventCoord, node)
		if distance < nearestDistance {
			nearestNode = node
			nearestDistance = distance
		}
	}

	if limit := maxNodeMatchDistanceMeters; limit > 0 && nearestDistance > limit {
		return types.Coordinate{}, false
	}

	return nearestNode, true
}
