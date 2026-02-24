// processor/segment_distribution.go

package processor

import (
	"main/src/lib"
	"main/src/types"
	"math"
)

// distributeSegmentTravelTime calculates per-node travel times between two
// matched events. Assumes uniform speed distribution across equally-spaced
// nodes (25m apart). Discards segments with invalid direction or unrealistic speed.
// The hour is derived from the previous event's timestamp to bucket results by time of day.
func DistributeSegmentTravelTime(
	prevEvent, currEvent *types.VehicleEvent,
	prevNodeIdx, currNodeIdx int,
	shapeID string,
	accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator,
) []types.NodeTravelTimeSampleRecord {
	var records []types.NodeTravelTimeSampleRecord
	
	nodeDelta, timeDelta, _, speedKmh, timePerNode := computeSegmentMetrics(prevEvent, currEvent, prevNodeIdx, currNodeIdx)

	// Enforce forward direction
	if nodeDelta <= 0 {
		return nil
	}

	if timeDelta <= 0 {
		return nil
	}

	const maxSpeedKmh = 120.0 // Upper bound for urban transit
	const minSpeedKmh = 1.0   // Filters stale/stuck GPS

	if speedKmh > maxSpeedKmh || speedKmh < minSpeedKmh {
		return nil
	}

	// Hour-of-day from epoch seconds
	hour := lib.GetHourFromTimestamp(int64(prevEvent.CreatedAt))
	
	shapeAcc, exists := accumulators[shapeID]
	if !exists {
		shapeAcc = make(map[types.NodeHourKey]*types.NodeAccumulator)
		accumulators[shapeID] = shapeAcc
	}

	// Distribute time to nodes [prevNodeIdx, currNodeIdx).
	// Each node's value = time to traverse from this node to the next.
	for nodeIdx := prevNodeIdx; nodeIdx < currNodeIdx; nodeIdx++ {
		records = append(records, types.NodeTravelTimeSampleRecord{
			ShapeID: shapeID,
			NodeIndex: nodeIdx,
			Hour: hour,
			Latitude: prevEvent.Latitude,
			Longitude: prevEvent.Longitude,
			CreatedAt: prevEvent.CreatedAt,
			TravelTimeSeconds: float32(math.Round(timePerNode)),
			SpeedKmh: speedKmh,
		})
		key := types.NodeHourKey{NodeIdx: nodeIdx, Hour: hour}
		acc, exists := shapeAcc[key]
		if !exists {
			acc = &types.NodeAccumulator{}
			shapeAcc[key] = acc
		}
		acc.Samples = append(acc.Samples, timePerNode)
	}

	return records
}

// ComputeSegmentMetrics is exported for use by tests in processor/tests.
func ComputeSegmentMetrics(prevEvent, currEvent *types.VehicleEvent, prevNodeIdx, currNodeIdx int) (nodeDelta int, timeDelta float64, distanceMeters float64, speedKmh float64, timePerNode float64) {
	return computeSegmentMetrics(prevEvent, currEvent, prevNodeIdx, currNodeIdx)
}

func computeSegmentMetrics(prevEvent, currEvent *types.VehicleEvent, prevNodeIdx, currNodeIdx int) (nodeDelta int, timeDelta float64, distanceMeters float64, speedKmh float64, timePerNode float64) {
	//
	
	// Calculate node delta
	nodeDelta = currNodeIdx - prevNodeIdx
	
	// Calculate time delta
	timeDelta = (float64(currEvent.CreatedAt) - float64(prevEvent.CreatedAt)) / 1000.0 // Convert to seconds
	
	// Calculate distance meters
	distanceMeters = float64(nodeDelta) * 25.0                                         // 25 meters per node
	
	// Calculate speed km/h
	speedKmh = (distanceMeters / timeDelta) * 3.6                                      // Convert to km/h
	
	// Calculate time per node
	timePerNode = timeDelta / float64(nodeDelta)
	
	return
}