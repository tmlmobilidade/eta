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
	shapeSamples := make([]types.NodeTravelTimeSampleRecord, 0)

	for lineID, lineShape := range lineShapesMap {
		//

		
		geohashes := lib.SetToSlice(lineShape.Geohashes)
		vehicleEvents := clickhouseClient.FetchVehicleEvents(ctx, geohashes, settings)
		lib.AppLogger.Info("Found %d vehicle events for line %d", len(vehicleEvents), lineID)

		shapeIDs := hashedShapesByLine.GetShapesByLineID(lineID)
		lib.AppLogger.Info("Found %d shapes for line %d", len(shapeIDs), lineID)

		for i, shapeID := range shapeIDs {
			lib.AppLogger.Info("Processing shape %d of %d: %s", i, len(shapeIDs), shapeID)
			lib.AppLogger.Info("Found %d vehicle events for shape %s", len(vehicleEvents), shapeID)

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

				if eventsBearing == -1 && prevEventNode != currEventNode{
					continue;
				}

				if !geo.IsValidBearing(eventsBearing, nodesBearing, settings.BearingThreshold) {
					continue;
				}

				prevNodeIdx := slices.Index(shapeNodes, prevEventNode)
				currNodeIdx := slices.Index(shapeNodes, currEventNode)

				records := DistributeSegmentTravelTime(
					&prevEvent, &currEvent,
					prevNodeIdx,
					currNodeIdx,
					shapeID, accumulators,
				)
				
				shapeSamples = append(shapeSamples, records...)
			}
			
			clickhouseClient.InsertNodeTravelTimeSamples(ctx, shapeSamples)
		}

	}

	// Build per-node travel time records keyed by shapeID + node + hour.
	shapeNodesByShapeID := make(map[string][]types.Coordinate)
	for _, lineShape := range lineShapesMap {
		maps.Copy(shapeNodesByShapeID, lineShape.Nodes)
	}

	return BuildRecords(accumulators, shapeNodesByShapeID)
}