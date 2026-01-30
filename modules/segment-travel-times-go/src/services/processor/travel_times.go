package processor

import (
	"main/src/lib"
	"main/src/lib/geo"
	"main/src/types"
)

// nodeEventMatch represents a matched event to a node.
type nodeEventMatch struct {
	nodeIndex int
	createdAt int64
	hour      int
}

// nodeTravelTimeSample represents a single travel time sample.
type nodeTravelTimeSample struct {
	nodeIndex         int
	hour              int
	travelTimeSeconds float64
}

// aggregatedSample represents aggregated travel time data for a node-hour combination.
type aggregatedSample struct {
	nodeIndex       int
	hour            int
	sampleCount     int
	totalTravelTime float64
}

// processLine calculates and aggregates travel times across all shapes for a given line.
func (lp *LineProcessor) processLine(vehicleEvents []types.VehicleEvent, lineID int, lineData *types.LineShapeData) []types.NodeTravelTimeRecord {
	var allRecords []types.NodeTravelTimeRecord
	var processedShapes, totalSamples int

	for _, hashedShapeID := range lineData.HashedShapeIDs {
		shapeNodes, exists := lineData.Nodes[hashedShapeID]
		if !exists || len(shapeNodes) < 2 {
			continue
		}

		// Calculate travel times for this shape
		records := lp.processShapeTravelTimes(
			vehicleEvents,
			shapeNodes,
			lineID,
			hashedShapeID,
		)

		if len(records) > 0 {
			allRecords = append(allRecords, records...)
			processedShapes++
			for _, r := range records {
				totalSamples += int(r.SampleCount)
			}
		}
	}

	lib.AppLogger.Debug("Processed %d/%d shapes with %d total samples",
		processedShapes, len(lineData.HashedShapeIDs), totalSamples)

	return allRecords
}

// processShapeTravelTimes processes all vehicle events for a shape and calculates travel times.
func (lp *LineProcessor) processShapeTravelTimes(events []types.VehicleEvent, nodes []types.Coordinate, lineID int, hashedShapeID string) []types.NodeTravelTimeRecord {
	if len(events) == 0 || len(nodes) < 2 {
		return nil
	}

	// Calculate bearings for all node pairs
	shapeBearings := geo.CalculateShapeBearings(nodes)

	// Group events by trip
	tripGroups := groupEventsByTrip(events)

	// Collect all samples from all trips
	var allSamples []nodeTravelTimeSample

	for _, tripEvents := range tripGroups {
		// Skip trips with only one event
		if len(tripEvents) < 2 {
			continue
		}

		// Match events to nodes, filtering by bearing
		matches := matchEventsToNodes(tripEvents, nodes, shapeBearings, lp.settings.BearingThreshold)

		// Calculate travel times from matches
		samples := calculateTravelTimeSamples(matches)
		allSamples = append(allSamples, samples...)
	}

	// Aggregate samples by node and hour
	aggregated := aggregateSamples(allSamples)

	// Convert to records for ClickHouse
	records := make([]types.NodeTravelTimeRecord, 0, len(aggregated))
	for _, data := range aggregated {
		node := nodes[data.nodeIndex]
		records = append(records, types.NodeTravelTimeRecord{
			LineID:            uint32(lineID),
			HashedShapeID:     hashedShapeID,
			NodeIndex:         uint16(data.nodeIndex),
			Latitude:          node.Latitude(),
			Longitude:         node.Longitude(),
			Hour:              uint8(data.hour),
			TravelTimeSeconds: float32(data.totalTravelTime / float64(data.sampleCount)),
			SampleCount:       uint32(data.sampleCount),
		})
	}

	return records
}
