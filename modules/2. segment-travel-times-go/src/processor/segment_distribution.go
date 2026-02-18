// processor/segment_distribution.go

package processor

import (
	"main/src/types"
)

// distributeSegmentTravelTime calculates per-node travel times between two
// matched events. Assumes uniform speed distribution across equally-spaced
// nodes (25m apart). Discards segments with invalid direction or unrealistic speed.
// The hour is derived from the previous event's timestamp to bucket results by time of day.
func distributeSegmentTravelTime(
	prevEvent, currEvent *types.VehicleEvent,
	prevNodeIdx, currNodeIdx int,
	shapeID string,
	accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator,
) {
	nodeDelta := currNodeIdx - prevNodeIdx

	// Enforce forward direction
	if nodeDelta <= 0 {
		return
	}

	timeDelta := float64(currEvent.CreatedAt - prevEvent.CreatedAt)
	if timeDelta <= 0 {
		return
	}

	// Speed validation in km/h
	distanceKm := float64(nodeDelta) * 0.025 // 25m per node
	speedKmh := (distanceKm / timeDelta) * 3600.0

	const maxSpeedKmh = 120.0 // Upper bound for urban transit
	const minSpeedKmh = 1.0   // Filters stale/stuck GPS

	if speedKmh > maxSpeedKmh || speedKmh < minSpeedKmh {
		return
	}

	timePerNode := timeDelta / float64(nodeDelta)

	// Hour-of-day from epoch seconds
	hour := uint8((prevEvent.CreatedAt / 3600) % 24)

	shapeAcc, exists := accumulators[shapeID]
	if !exists {
		shapeAcc = make(map[types.NodeHourKey]*types.NodeAccumulator)
		accumulators[shapeID] = shapeAcc
	}

	// Distribute time to nodes [prevNodeIdx, currNodeIdx).
	// Each node's value = time to traverse from this node to the next.
	for nodeIdx := prevNodeIdx; nodeIdx < currNodeIdx; nodeIdx++ {
		key := types.NodeHourKey{NodeIdx: nodeIdx, Hour: hour}
		acc, exists := shapeAcc[key]
		if !exists {
			acc = &types.NodeAccumulator{}
			shapeAcc[key] = acc
		}
		acc.Samples = append(acc.Samples, timePerNode)
	}
}
