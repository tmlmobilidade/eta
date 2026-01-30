package processor

import (
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
// Uses NodeIndex for O(1) average nearest-node lookups instead of O(n) linear scans.
func matchEventsToNodes(tripEvents []types.VehicleEvent, nodeIndex *geo.NodeIndex, shapeBearings []float64, bearingThreshold float64) []nodeEventMatch {
	if len(tripEvents) < 2 || len(shapeBearings) < 2 {
		return nil
	}

	// Pre-allocate with estimated capacity
	matches := make([]nodeEventMatch, 0, len(tripEvents))

	// Process consecutive event pairs to check bearing
	for i := 0; i < len(tripEvents)-1; i++ {
		currentEvent := tripEvents[i]
		nextEvent := tripEvents[i+1]

		// Calculate event bearing (direction of travel)
		eventBearing := geo.CalculateBearing(
			types.Coordinate{currentEvent.Longitude, currentEvent.Latitude},
			types.Coordinate{nextEvent.Longitude, nextEvent.Latitude},
		)

		// Find nearest node using geohash-bucketed index (O(1) average)
		nearestNodeIdx := nodeIndex.FindNearest(currentEvent.Longitude, currentEvent.Latitude)

		// Check if event bearing matches shape bearing at this node
		shapeBearing := shapeBearings[nearestNodeIdx]
		if !geo.IsValidBearing(eventBearing, shapeBearing, bearingThreshold) {
			// Event is traveling in wrong direction, skip
			continue
		}

		matches = append(matches, nodeEventMatch{
			nodeIndex: nearestNodeIdx,
			createdAt: currentEvent.CreatedAt,
			hour:      extractHour(currentEvent.CreatedAt),
		})
	}

	// Handle the last event if we have previous valid matches
	if len(matches) > 0 {
		lastEvent := tripEvents[len(tripEvents)-1]
		lastNodeIdx := nodeIndex.FindNearest(lastEvent.Longitude, lastEvent.Latitude)

		// Only add if it advances along the shape (prevents duplicates)
		lastMatch := matches[len(matches)-1]
		if lastNodeIdx > lastMatch.nodeIndex {
			matches = append(matches, nodeEventMatch{
				nodeIndex: lastNodeIdx,
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

	// Estimate capacity: roughly one sample per node gap between matches
	estimatedNodes := 0
	for i := 0; i < len(matches)-1; i++ {
		gap := matches[i+1].nodeIndex - matches[i].nodeIndex
		if gap > 0 {
			estimatedNodes += gap
		}
	}
	samples := make([]nodeTravelTimeSample, 0, estimatedNodes)

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

// makeAggregationKey creates a uint32 key from nodeIndex and hour.
// Layout: nodeIndex (bits 8-31) | hour (bits 0-7)
// This avoids string allocation overhead from fmt.Sprintf.
func makeAggregationKey(nodeIndex, hour int) uint32 {
	return uint32(nodeIndex)<<8 | uint32(hour&0xFF)
}

// aggregateSamples aggregates travel time samples by node index and hour.
// Uses integer keys instead of string keys to avoid allocation overhead.
func aggregateSamples(samples []nodeTravelTimeSample) map[uint32]*aggregatedSample {
	// Pre-allocate with estimated size (roughly samples / 2 for deduplication)
	estimatedSize := len(samples)/2 + 1
	aggregated := make(map[uint32]*aggregatedSample, estimatedSize)

	for _, sample := range samples {
		key := makeAggregationKey(sample.nodeIndex, sample.hour)

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
