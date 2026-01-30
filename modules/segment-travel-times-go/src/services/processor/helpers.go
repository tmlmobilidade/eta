package processor

import (
	"fmt"
	"main/src/lib/geo"
	"main/src/types"
)

// groupEventsByTrip groups vehicle events by their TripOperationalID.
func groupEventsByTrip(events []types.VehicleEvent) map[string][]types.VehicleEvent {
	grouped := make(map[string][]types.VehicleEvent)

	for _, event := range events {
		tripID := event.TripOperationalID
		grouped[tripID] = append(grouped[tripID], event)
	}

	return grouped
}

// extractHour extracts the hour (0-23) from a Unix timestamp in milliseconds.
func extractHour(unixTimestampMs int64) int {
	// Convert to seconds and get hours (UTC)
	seconds := unixTimestampMs / 1000
	return int((seconds / 3600) % 24)
}

// matchEventsToNodes filters and matches vehicle events to shape nodes based on bearing.
func matchEventsToNodes(tripEvents []types.VehicleEvent, nodes []types.Coordinate, shapeBearings []float64, bearingThreshold float64) []nodeEventMatch {
	if len(tripEvents) < 2 || len(nodes) < 2 {
		return nil
	}

	var matches []nodeEventMatch

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

		matches = append(matches, nodeEventMatch{
			nodeIndex: nodeIndex,
			createdAt: currentEvent.CreatedAt,
			hour:      extractHour(currentEvent.CreatedAt),
		})
	}

	// Handle the last event if we have previous valid matches
	if len(matches) > 0 {
		lastEvent := tripEvents[len(tripEvents)-1]
		lastNodeIndex := geo.FindNearestNodeIndex(lastEvent.Longitude, lastEvent.Latitude, nodes)

		// Only add if it advances along the shape (prevents duplicates)
		lastMatch := matches[len(matches)-1]
		if lastNodeIndex > lastMatch.nodeIndex {
			matches = append(matches, nodeEventMatch{
				nodeIndex: lastNodeIndex,
				createdAt: lastEvent.CreatedAt,
				hour:      extractHour(lastEvent.CreatedAt),
			})
		}
	}

	return matches
}

// calculateTravelTimeSamples calculates travel times from matched events.
func calculateTravelTimeSamples(matches []nodeEventMatch) []nodeTravelTimeSample {
	if len(matches) < 2 {
		return nil
	}

	var samples []nodeTravelTimeSample

	for i := 0; i < len(matches)-1; i++ {
		startMatch := matches[i]
		endMatch := matches[i+1]

		// Skip if nodes are not advancing
		if endMatch.nodeIndex <= startMatch.nodeIndex {
			continue
		}

		// Convert from milliseconds to seconds
		timeDiffMs := endMatch.createdAt - startMatch.createdAt
		timeDiffSeconds := float64(timeDiffMs) / 1000.0
		nodeCount := endMatch.nodeIndex - startMatch.nodeIndex

		// Distribute time evenly across all nodes in the segment
		timePerNode := timeDiffSeconds / float64(nodeCount)

		// Assign travel time to each node in the segment (excluding the start node)
		for nodeIdx := startMatch.nodeIndex + 1; nodeIdx <= endMatch.nodeIndex; nodeIdx++ {
			samples = append(samples, nodeTravelTimeSample{
				nodeIndex:         nodeIdx,
				hour:              startMatch.hour,
				travelTimeSeconds: timePerNode,
			})
		}
	}

	return samples
}

// aggregateSamples aggregates travel time samples by node index and hour.
func aggregateSamples(samples []nodeTravelTimeSample) map[string]*aggregatedSample {
	aggregated := make(map[string]*aggregatedSample)

	for _, sample := range samples {
		key := fmt.Sprintf("%d-%d", sample.nodeIndex, sample.hour)

		if existing, exists := aggregated[key]; exists {
			existing.sampleCount++
			existing.totalTravelTime += sample.travelTimeSeconds
		} else {
			aggregated[key] = &aggregatedSample{
				nodeIndex:       sample.nodeIndex,
				hour:            sample.hour,
				sampleCount:     1,
				totalTravelTime: sample.travelTimeSeconds,
			}
		}
	}

	return aggregated
}
