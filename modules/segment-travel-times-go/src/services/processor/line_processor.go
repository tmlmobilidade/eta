package processor

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/lib/geo"
	clickhouse "main/src/services/clickhouse"
	"main/src/types"
	"sort"
	"sync"
)

// LineProcessor handles parallel processing of lines for travel time calculation.
type LineProcessor struct {
	clickhouse *clickhouse.ClickhouseService
	settings   *types.Settings
}

// NewLineProcessor creates a new LineProcessor instance.
func NewLineProcessor(ch *clickhouse.ClickhouseService, settings *types.Settings) *LineProcessor {
	return &LineProcessor{
		clickhouse: ch,
		settings:   settings,
	}
}

// lineJob represents a single line processing job for the worker pool.
type lineJob struct {
	index    int
	lineID   int
	lineData *types.LineShapeData
}

// lineResult represents the result of processing a single line.
type lineResult struct {
	index      int
	lineID     int
	records    []types.NodeTravelTimeRecord
	err        error
	skipped    bool
	skipReason string
}

// ProcessAllLines processes all lines using a worker pool for parallel execution.
// Uses goroutines with a configurable worker count for optimal performance.
func (lp *LineProcessor) ProcessAllLines(ctx context.Context, lineShapes types.LineShapesMap) error {
	// Sort lines by lineID for consistent ordering
	sortedLines := lp.sortLines(lineShapes)
	totalLines := len(sortedLines)

	if totalLines == 0 {
		lib.AppLogger.Info("No lines to process")
		return nil
	}

	lib.AppLogger.Title(fmt.Sprintf("Processing %d lines with %d workers", totalLines, lp.settings.WorkerCount))

	// Create channels for job distribution and result collection
	jobs := make(chan lineJob, totalLines)
	results := make(chan lineResult, totalLines)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < lp.settings.WorkerCount; w++ {
		wg.Add(1)
		go lp.worker(ctx, w, jobs, results, &wg, totalLines)
	}

	// Submit all jobs
	for index, line := range sortedLines {
		jobs <- lineJob{
			index:    index,
			lineID:   line.lineID,
			lineData: line.data,
		}
	}
	close(jobs)

	// Wait for all workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and process results
	var totalRecords, processedLines, skippedLines int
	var firstError error

	for result := range results {
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			lib.AppLogger.Error(result.err, "Failed to process line %d", result.lineID)
			continue
		}

		if result.skipped {
			skippedLines++
			continue
		}

		// Save records to ClickHouse
		if len(result.records) > 0 {
			if err := lp.clickhouse.SaveTravelTimes(ctx, result.records); err != nil {
				lib.AppLogger.Error(err, "Failed to save travel times for line %d", result.lineID)
				if firstError == nil {
					firstError = err
				}
				continue
			}
			totalRecords += len(result.records)
		}

		processedLines++
	}

	lib.AppLogger.Success(
		"Processing complete: %d lines processed, %d skipped, %d total records saved",
		processedLines, skippedLines, totalRecords,
	)

	return firstError
}

// sortedLine is a helper struct for sorting lines by ID.
type sortedLine struct {
	lineID int
	data   *types.LineShapeData
}

// sortLines converts the map to a sorted slice for consistent ordering.
func (lp *LineProcessor) sortLines(lineShapes types.LineShapesMap) []sortedLine {
	sorted := make([]sortedLine, 0, len(lineShapes))
	for lineID, data := range lineShapes {
		sorted = append(sorted, sortedLine{lineID: lineID, data: data})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].lineID < sorted[j].lineID
	})
	return sorted
}

// worker processes jobs from the jobs channel and sends results to the results channel.
func (lp *LineProcessor) worker(
	ctx context.Context,
	workerID int,
	jobs <-chan lineJob,
	results chan<- lineResult,
	wg *sync.WaitGroup,
	totalLines int,
) {
	defer wg.Done()

	for job := range jobs {
		result := lp.processLineJob(ctx, job, totalLines)
		results <- result
	}
}

// processLineJob processes a single line job and returns the result.
func (lp *LineProcessor) processLineJob(ctx context.Context, job lineJob, totalLines int) lineResult {
	lineID := job.lineID
	lineData := job.lineData

	lib.AppLogger.Info("[%d/%d] Processing line %d with %d shapes and %d geohashes",
		job.index+1, totalLines, lineID, len(lineData.HashedShapeIDs), len(lineData.Geohashes))

	// Fetch vehicle events for this line's geohashes
	vehicleEvents, err := lp.fetchAndPrepareVehicleEvents(ctx, lineID, lineData)
	if err != nil {
		return lineResult{index: job.index, lineID: lineID, err: err}
	}

	if vehicleEvents == nil {
		lib.AppLogger.Info("[%d/%d] No vehicle events found for line %d, skipping",
			job.index+1, totalLines, lineID)
		return lineResult{
			index:      job.index,
			lineID:     lineID,
			skipped:    true,
			skipReason: "no vehicle events",
		}
	}

	lib.AppLogger.Info("[%d/%d] Found %d vehicle events for line %d",
		job.index+1, totalLines, len(vehicleEvents), lineID)

	// Process the line
	records := lp.processLine(vehicleEvents, lineID, lineData)

	lib.AppLogger.Info("[%d/%d] Generated %d travel time records for line %d",
		job.index+1, totalLines, len(records), lineID)

	return lineResult{
		index:   job.index,
		lineID:  lineID,
		records: records,
	}
}

// fetchAndPrepareVehicleEvents fetches vehicle events and prepares the line for processing.
func (lp *LineProcessor) fetchAndPrepareVehicleEvents(
	ctx context.Context,
	lineID int,
	lineData *types.LineShapeData,
) ([]types.VehicleEvent, error) {
	// Convert geohash set to slice
	geohashes := make([]string, 0, len(lineData.Geohashes))
	for gh := range lineData.Geohashes {
		geohashes = append(geohashes, gh)
	}

	// Fetch vehicle events
	vehicleEvents, err := lp.clickhouse.FetchVehicleEvents(ctx, geohashes, lp.settings)
	if err != nil {
		return nil, err
	}

	if len(vehicleEvents) == 0 {
		return nil, nil
	}

	// Delete existing travel time records to prevent duplicates
	if err := lp.clickhouse.DeleteTravelTimesForShapes(ctx, uint32(lineID), lineData.HashedShapeIDs); err != nil {
		return nil, err
	}

	return vehicleEvents, nil
}

// processLine calculates and aggregates travel times across all shapes for a given line.
func (lp *LineProcessor) processLine(
	vehicleEvents []types.VehicleEvent,
	lineID int,
	lineData *types.LineShapeData,
) []types.NodeTravelTimeRecord {
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

	lib.AppLogger.Info("Processed %d/%d shapes with %d total samples",
		processedShapes, len(lineData.HashedShapeIDs), totalSamples)

	return allRecords
}

// processShapeTravelTimes processes all vehicle events for a shape and calculates travel times.
func (lp *LineProcessor) processShapeTravelTimes(
	events []types.VehicleEvent,
	nodes []types.Coordinate,
	lineID int,
	hashedShapeID string,
) []types.NodeTravelTimeRecord {
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

// =============================================================================
// Travel Time Calculation Helper Types and Functions
// =============================================================================

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
func matchEventsToNodes(
	tripEvents []types.VehicleEvent,
	nodes []types.Coordinate,
	shapeBearings []float64,
	bearingThreshold float64,
) []nodeEventMatch {
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
