package processor

import (
	"context"
	"main/src/lib"
	"main/src/lib/geo"
	services "main/src/services/clickhouse"
	"main/src/types"
	"slices"
)

type LinesProcessor struct {
	clickhouse *services.ClickhouseService
	settings *types.Settings
}

func NewLinesProcessor(clickhouse *services.ClickhouseService, settings *types.Settings) *LinesProcessor {
	return &LinesProcessor{clickhouse: clickhouse, settings: settings}
}

func (p *LinesProcessor) ProcessAllLines(ctx context.Context, lineShapes types.LineShapesMap) error {
	for _, value := range lineShapes {
		err := p.processLine(ctx, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *LinesProcessor) processLine(ctx context.Context, lineData *types.LineShapeData) error {

	// Fetch vehicle events for the line
	geohashes := lib.SetToSlice(lineData.Geohashes)
	vehicleEvents, err := p.clickhouse.FetchVehicleEvents(ctx, geohashes, p.settings)
	if err != nil {
		return err
	}


	for _, shapeID := range lineData.HashedShapeIDs {
		//
		
		lib.AppLogger.Debug("Processing shape %s", shapeID)

		// Get shape nodes for this shape
		shapeNodes, exists := lineData.Nodes[shapeID]

		// Skip shapes with less than 2 nodes
		if !exists || len(shapeNodes) < 2 {
			continue
		}
		
		// Calculate Travel Times for this shape by iterating over the vehicle events
		// We calculate groupings of the same trip ID, this is so we can get a bearing of events
		for i := 1; i < len(vehicleEvents); i++ {
			//

			currEvent := vehicleEvents[i]
			prevEvent := vehicleEvents[i-1]

			// Skip if the current event is not the same trip as the previous event
			// This means we are on a new trip ID
			if currEvent.TripOperationalID != prevEvent.TripOperationalID {
				continue
			}

			// Match events to nodes with a maximum distance threshold (in meters).
			prevEventNode, okPrev := p.matchEventToNode(prevEvent, shapeNodes)
			currEventNode, okCurr := p.matchEventToNode(currEvent, shapeNodes)

			// Skip if either event is too far from all nodes.
			if !okPrev || !okCurr {
				continue
			}

			// Calculate bearing of events and nodes
			eventsBearing := geo.CalculateBearing(
				types.Coordinate{prevEvent.Longitude, prevEvent.Latitude},
				types.Coordinate{currEvent.Longitude, currEvent.Latitude},
			)

			nodesBearing := geo.CalculateBearing(prevEventNode, currEventNode)

			if (prevEvent.TripOperationalID == "1001_0_1_0600_0629_0_1-20260204") {

			// Check if nodes are in valid bearing difference
			lib.AppLogger.Accent(`
				Events: %v, %v
				Nodes: %v, %v
				Events Bearing: %v
				Nodes Bearing: %v
				Nodes Index: %d, %d
				Valid Bearing: %v
			`, prevEvent, currEvent, prevEventNode, currEventNode, eventsBearing, nodesBearing,  slices.Index(shapeNodes, prevEventNode), slices.Index(shapeNodes, currEventNode), geo.IsValidBearing(eventsBearing, nodesBearing, p.settings.BearingThreshold))
			}

			if !geo.IsValidBearing(eventsBearing, nodesBearing, p.settings.BearingThreshold) {
				continue
			}

		}
	}
	

	return nil
}

func (p *LinesProcessor) matchEventToNode(event types.VehicleEvent, nodes []types.Coordinate) (types.Coordinate, bool) {
	// Find the nearest node to the event and enforce an optional maximum distance limit (in meters).
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

	// If a maximum distance is configured (> 0), only accept matches within that limit.
	if limit := p.settings.MaxNodeMatchDistanceMeters; limit > 0 && nearestDistance > limit {
		return types.Coordinate{}, false
	}

	return nearestNode, true
}