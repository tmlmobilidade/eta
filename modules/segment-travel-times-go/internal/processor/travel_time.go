package processor

import (
	"fmt"
	"sync"
	"time"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/geo"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

// groupEventsByTrip groups vehicle events by their trip_operational_id.
// Events within each group are already sorted by created_at (from ClickHouse query).
func groupEventsByTrip(events []types.VehicleEvent) map[string][]types.VehicleEvent {
	grouped := make(map[string][]types.VehicleEvent)

	for _, event := range events {
		grouped[event.TripOperationalID] = append(grouped[event.TripOperationalID], event)
	}

	return grouped
}

// extractHour extracts the hour (0-23) from a Unix timestamp in milliseconds.
// Uses UTC for consistency.
func extractHour(unixTimestampMs int64) int {
	return time.UnixMilli(unixTimestampMs).UTC().Hour()
}

// matchEventsToNodes filters and matches vehicle events to shape nodes based on bearing.
// Only events traveling in the same direction as the shape are included.
func matchEventsToNodes(
	tripEvents []types.VehicleEvent,
	nodes []types.Coordinate,
	shapeBearings []float64,
	bearingThreshold float64,
) []types.NodeEventMatch {
	if len(tripEvents) < 2 || len(nodes) < 2 {
		return []types.NodeEventMatch{}
	}

	matches := []types.NodeEventMatch{}

	// Process consecutive event pairs to check bearing
	for i := 0; i < len(tripEvents)-1; i++ {
		currentEvent := tripEvents[i]
		nextEvent := tripEvents[i+1]

		// Calculate event bearing (direction of travel)
		eventBearing := geo.CalculateBearing(
			types.Coordinate{currentEvent.Longitude, currentEvent.Latitude},
			types.Coordinate{nextEvent.Longitude, nextEvent.Latitude},
		)

		// Find nearest node to the current event
		nodeIndex := geo.FindNearestNodeIndex(currentEvent.Longitude, currentEvent.Latitude, nodes)

		// Check if event bearing matches shape bearing at this node
		shapeBearing := shapeBearings[nodeIndex]
		if !geo.IsValidBearing(eventBearing, shapeBearing, bearingThreshold) {
			// Event is traveling in wrong direction, skip
			continue
		}

		matches = append(matches, types.NodeEventMatch{
			CreatedAt:       currentEvent.CreatedAt,
			Hour:            extractHour(currentEvent.CreatedAt),
			NodeIndex:       nodeIndex,
			OperationalDate: currentEvent.OperationalDate,
		})
	}

	// Handle the last event if we have previous valid matches
	if len(matches) > 0 {
		lastEvent := tripEvents[len(tripEvents)-1]
		lastNodeIndex := geo.FindNearestNodeIndex(lastEvent.Longitude, lastEvent.Latitude, nodes)

		// Only add if it advances along the shape (prevents duplicates)
		lastMatch := matches[len(matches)-1]
		if lastNodeIndex > lastMatch.NodeIndex {
			matches = append(matches, types.NodeEventMatch{
				CreatedAt:       lastEvent.CreatedAt,
				Hour:            extractHour(lastEvent.CreatedAt),
				NodeIndex:       lastNodeIndex,
				OperationalDate: lastEvent.OperationalDate,
			})
		}
	}

	return matches
}

// calculateTravelTimeSamples calculates travel times from matched events and distributes them across nodes.
// If events match nodes A and D with B and C in between, the travel time is
// distributed evenly across all nodes from A to D.
func calculateTravelTimeSamples(matches []types.NodeEventMatch) []types.NodeTravelTimeSample {
	if len(matches) < 2 {
		return []types.NodeTravelTimeSample{}
	}

	samples := []types.NodeTravelTimeSample{}

	for i := 0; i < len(matches)-1; i++ {
		startMatch := matches[i]
		endMatch := matches[i+1]

		// Skip if nodes are not advancing (vehicle might have stopped or gone backwards)
		if endMatch.NodeIndex <= startMatch.NodeIndex {
			continue
		}

		// Convert from milliseconds to seconds
		timeDiffMs := endMatch.CreatedAt - startMatch.CreatedAt
		timeDiffSeconds := float64(timeDiffMs) / 1000.0
		nodeCount := endMatch.NodeIndex - startMatch.NodeIndex

		// Distribute time evenly across all nodes in the segment
		timePerNode := timeDiffSeconds / float64(nodeCount)

		// Assign travel time to each node in the segment (excluding the start node)
		for nodeIdx := startMatch.NodeIndex + 1; nodeIdx <= endMatch.NodeIndex; nodeIdx++ {
			samples = append(samples, types.NodeTravelTimeSample{
				Hour:              startMatch.Hour,
				NodeIndex:         nodeIdx,
				TravelTimeSeconds: timePerNode,
			})
		}
	}

	return samples
}

// aggregateSamples aggregates travel time samples by node index and hour.
// Returns a map keyed by "nodeIndex-hour" with aggregated data.
func aggregateSamples(samples []types.NodeTravelTimeSample) map[string]*types.AggregatedSample {
	aggregated := make(map[string]*types.AggregatedSample)

	for _, sample := range samples {
		key := fmt.Sprintf("%d-%d", sample.NodeIndex, sample.Hour)
		existing, found := aggregated[key]

		if found {
			existing.SampleCount++
			existing.TotalTravelTime += sample.TravelTimeSeconds
		} else {
			aggregated[key] = &types.AggregatedSample{
				Hour:            sample.Hour,
				NodeIndex:       sample.NodeIndex,
				SampleCount:     1,
				TotalTravelTime: sample.TravelTimeSeconds,
			}
		}
	}

	return aggregated
}

// processShapeTravelTimes processes all vehicle events for a shape and calculates travel times.
// This is the core calculation function for a single shape.
func processShapeTravelTimes(
	events []types.VehicleEvent,
	nodes []types.Coordinate,
	lineID int,
	hashedShapeID string,
	bearingThreshold float64,
) []types.NodeTravelTimeRecord {
	if len(events) == 0 || len(nodes) < 2 {
		return []types.NodeTravelTimeRecord{}
	}

	// Calculate bearings for all node pairs
	shapeBearings := geo.CalculateShapeBearings(nodes)

	// Group events by trip
	tripGroups := groupEventsByTrip(events)

	// Collect all samples from all trips
	allSamples := []types.NodeTravelTimeSample{}

	for _, tripEvents := range tripGroups {
		// Skip trips with only one event
		if len(tripEvents) < 2 {
			continue
		}

		// Match events to nodes, filtering by bearing
		matches := matchEventsToNodes(tripEvents, nodes, shapeBearings, bearingThreshold)

		// Calculate travel times from matches
		samples := calculateTravelTimeSamples(matches)
		allSamples = append(allSamples, samples...)
	}

	// Aggregate samples by node and hour
	aggregated := aggregateSamples(allSamples)

	// Convert to records for ClickHouse
	records := make([]types.NodeTravelTimeRecord, 0, len(aggregated))
	for _, data := range aggregated {
		node := nodes[data.NodeIndex]
		records = append(records, types.NodeTravelTimeRecord{
			HashedShapeID:     hashedShapeID,
			Hour:              data.Hour,
			Latitude:          node.Latitude(),
			LineID:            lineID,
			Longitude:         node.Longitude(),
			NodeIndex:         data.NodeIndex,
			SampleCount:       data.SampleCount,
			TravelTimeSeconds: data.TotalTravelTime / float64(data.SampleCount),
		})
	}

	return records
}

// ProcessShapeTravelTimesConcurrent processes multiple shapes concurrently using goroutines.
// This provides shape-level parallelism within a single line.
func ProcessShapeTravelTimesConcurrent(
	events []types.VehicleEvent,
	lineData *types.LineShapeData,
	lineID int,
	bearingThreshold float64,
) []types.NodeTravelTimeRecord {
	type shapeResult struct {
		records []types.NodeTravelTimeRecord
	}

	resultsChan := make(chan shapeResult, len(lineData.HashedShapeIDs))
	var wg sync.WaitGroup

	for _, shapeID := range lineData.HashedShapeIDs {
		nodes, exists := lineData.Nodes[shapeID]
		if !exists || len(nodes) < 2 {
			continue
		}

		wg.Add(1)
		go func(shapeID string, nodes []types.Coordinate) {
			defer wg.Done()
			records := processShapeTravelTimes(events, nodes, lineID, shapeID, bearingThreshold)
			resultsChan <- shapeResult{records: records}
		}(shapeID, nodes)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect all results
	var allRecords []types.NodeTravelTimeRecord
	for result := range resultsChan {
		allRecords = append(allRecords, result.records...)
	}

	return allRecords
}
