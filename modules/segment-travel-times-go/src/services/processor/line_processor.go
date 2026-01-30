package processor

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"sort"
	"sync"
)

// ClickhouseClient defines the ClickHouse operations needed by LineProcessor.
type ClickhouseClient interface {
	FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error)
	DeleteTravelTimesForShapes(ctx context.Context, lineID uint32, hashedShapeIDs []string) error
	SaveTravelTimes(ctx context.Context, records []types.NodeTravelTimeRecord) error
}

// LineProcessor handles parallel processing of lines for travel time calculation.
type LineProcessor struct {
	clickhouse ClickhouseClient
	settings   *types.Settings
}

// NewLineProcessor creates a new LineProcessor instance.
func NewLineProcessor(ch ClickhouseClient, settings *types.Settings) *LineProcessor {
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

	// Create worker display
	display := lib.NewWorkerDisplay(lp.settings.WorkerCount, totalLines)

	// Create channels for job distribution and result collection
	jobs := make(chan lineJob, totalLines)
	results := make(chan lineResult, totalLines)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < lp.settings.WorkerCount; w++ {
		wg.Add(1)
		go lp.worker(ctx, w, jobs, results, &wg, display)
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
			display.LineErrored()
			lib.AppLogger.Error(result.err, "Failed to process line %d", result.lineID)
			continue
		}

		if result.skipped {
			skippedLines++
			display.LineSkipped()
			continue
		}

		// Save records to ClickHouse
		if len(result.records) > 0 {
			if err := lp.clickhouse.SaveTravelTimes(ctx, result.records); err != nil {
				lib.AppLogger.Error(err, "Failed to save travel times for line %d", result.lineID)
				if firstError == nil {
					firstError = err
				}
				display.LineErrored()
				continue
			}
			totalRecords += len(result.records)
		}

		processedLines++
		display.LineCompleted(len(result.records))
	}

	// Stop display
	display.Stop()

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
func (lp *LineProcessor) worker(ctx context.Context, workerID int, jobs <-chan lineJob, results chan<- lineResult, wg *sync.WaitGroup, display *lib.WorkerDisplay) {
	defer wg.Done()
	defer display.IdleWorker(workerID)

	for job := range jobs {
		result := lp.processLineJob(ctx, job, workerID, display)
		results <- result
	}
}

// processLineJob processes a single line job and returns the result.
func (lp *LineProcessor) processLineJob(ctx context.Context, job lineJob, workerID int, display *lib.WorkerDisplay) lineResult {
	lineID := job.lineID
	lineData := job.lineData

	// Update display: fetching events
	display.UpdateWorker(workerID, lineID,
		fmt.Sprintf("Fetching events (%d geohashes)", len(lineData.Geohashes)))

	// Fetch vehicle events for this line's geohashes
	vehicleEvents, err := lp.fetchAndPrepareVehicleEvents(ctx, lineID, lineData)
	if err != nil {
		return lineResult{index: job.index, lineID: lineID, err: err}
	}

	if vehicleEvents == nil {
		return lineResult{
			index:      job.index,
			lineID:     lineID,
			skipped:    true,
			skipReason: "no vehicle events",
		}
	}

	// Update display: processing
	display.UpdateWorker(workerID, lineID,
		fmt.Sprintf("Processing %s events (%d shapes)",
			lib.FormatCount(len(vehicleEvents)), len(lineData.HashedShapeIDs)))

	// Process the line
	records := lp.processLine(vehicleEvents, lineID, lineData)

	return lineResult{
		index:   job.index,
		lineID:  lineID,
		records: records,
	}
}

// fetchAndPrepareVehicleEvents fetches vehicle events and prepares the line for processing.
func (lp *LineProcessor) fetchAndPrepareVehicleEvents(ctx context.Context, lineID int, lineData *types.LineShapeData) ([]types.VehicleEvent, error) {
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
