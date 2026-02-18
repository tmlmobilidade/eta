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
// Returns NodeTravelTimeRecord entries keyed by shapeID + node + hour.
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

	return BuildRecords(accumulators, shapeNodesByShapeID)
}