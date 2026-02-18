// processor/travel_times.go

package processor

import (
	"context"
	"maps"
	"slices"

	"main/src/lib"
	"main/src/lib/geo"
	clickhouseService "main/src/services/clickhouse"
	"main/src/types"
)

// ProcessLineShapes processes line shapes to calculate travel times from vehicle events.
// Returns accumulators keyed by shapeID -> node+hour -> collected samples.
func ProcessLineShapes(
	ctx context.Context,
	clickhouseClient *clickhouseService.ClickhouseClient,
	lineShapesMap types.LineShapesMap,
	hashedShapesByLine *types.HashedShapeIdsByLineArray,
	settings *types.Settings,
) []types.NodeTravelTimeRecord {

	accumulators := make(map[string]map[types.NodeHourKey]*types.NodeAccumulator)

	for lineID, lineShape := range lineShapesMap {

		geohashes := lib.SetToSlice(lineShape.Geohashes)
		vehicleEvents := clickhouseClient.FetchVehicleEvents(ctx, geohashes, settings)
		lib.AppLogger.Info("Found %d vehicle events for line %d", len(vehicleEvents), lineID)

		for _, shapeID := range hashedShapesByLine.GetShapesByLineID(lineID) {

			shapeNodes, exists := lineShape.Nodes[shapeID]

			if !exists || len(shapeNodes) < 2 {
				continue
			}

			for i := 1; i < len(vehicleEvents); i++ {

				currEvent := vehicleEvents[i]
				prevEvent := vehicleEvents[i-1]

				if currEvent.RideId != prevEvent.RideId {
					continue
				}

				prevEventNode, okPrev := MatchEventToNode(prevEvent, shapeNodes, settings.MaxNodeMatchDistanceMeters)
				currEventNode, okCurr := MatchEventToNode(currEvent, shapeNodes, settings.MaxNodeMatchDistanceMeters)

				if !okPrev || !okCurr {
					continue
				}

				eventsBearing := geo.CalculateBearing(
					types.Coordinate{prevEvent.Longitude, prevEvent.Latitude},
					types.Coordinate{currEvent.Longitude, currEvent.Latitude},
				)

				nodesBearing := geo.CalculateBearing(prevEventNode, currEventNode)

				if !geo.IsValidBearing(eventsBearing, nodesBearing, settings.BearingThreshold) && prevEventNode != currEventNode {
					lib.AppLogger.LogToFile("bearing_debug.log",
						"Events: \n 1: %v\n 2: %v\nNodes: \n 1: %v\n 2: %v\nEvents Bearing: %v\nNodes Bearing: %v\nNodes Index: %d, %d\nValid Bearing: %v",
						prevEvent,
						currEvent,
						prevEventNode,
						currEventNode,
						eventsBearing,
						nodesBearing,
						slices.Index(shapeNodes, prevEventNode),
						slices.Index(shapeNodes, currEventNode),
						geo.IsValidBearing(eventsBearing, nodesBearing, settings.BearingThreshold),
					)
					panic("An invalid bearing was found")
				}

				distributeSegmentTravelTime(
					&prevEvent, &currEvent,
					slices.Index(shapeNodes, prevEventNode),
					slices.Index(shapeNodes, currEventNode),
					shapeID, accumulators,
				)
			}
		}
	}

	// Build per-node travel time records keyed by shapeID + node + hour.
	shapeNodesByShapeID := make(map[string][]types.Coordinate)
	for _, lineShape := range lineShapesMap {
		maps.Copy(shapeNodesByShapeID, lineShape.Nodes)
	}

	records := BuildRecords(accumulators, shapeNodesByShapeID)

	return records
}

// distributeSegmentTravelTime calculates per-node travel times between two
// matched events. Assumes uniform speed distribution across equally-spaced
// nodes (25m apart). Discards segments with invalid direction or unrealistic speed.
// The hour is derived from the previous event's timestamp to bucket results by time of day.
func distributeSegmentTravelTime(prevEvent, currEvent *types.VehicleEvent, prevNodeIdx, currNodeIdx int, shapeID string, accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator) {
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

// BuildRecords converts accumulators into final NodeTravelTimeRecord entries.
// Produces one record per shape + node + hour combination.
func BuildRecords(
	accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator,
	shapeNodes map[string][]types.Coordinate,
) []types.NodeTravelTimeRecord {
	var records []types.NodeTravelTimeRecord

	for shapeID, nodeAccs := range accumulators {
		nodes := shapeNodes[shapeID]
		for key, acc := range nodeAccs {
			if len(acc.Samples) == 0 || key.NodeIdx >= len(nodes) {
				continue
			}
			records = append(records, types.NodeTravelTimeRecord{
				ShapeID:           shapeID,
				NodeIndex:         key.NodeIdx,
				Hour:              key.Hour,
				Latitude:          nodes[key.NodeIdx].Latitude(),
				Longitude:         nodes[key.NodeIdx].Longitude(),
				TravelTimeSeconds: float32(acc.Median()),
				SampleCount:       uint32(len(acc.Samples)),
			})
		}
	}

	return records
}