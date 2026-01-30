package processor

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// TimingStats tracks aggregate timing information for performance analysis.
type TimingStats struct {
	totalFetchTime   atomic.Int64 // nanoseconds spent fetching from ClickHouse
	totalProcessTime atomic.Int64 // nanoseconds spent processing (CPU-bound)
	totalSaveTime    atomic.Int64 // nanoseconds spent saving to ClickHouse
	linesProcessed   atomic.Int64
	eventsProcessed  atomic.Int64
	cacheHits        atomic.Int64 // geohashes found in cache
	cacheMisses      atomic.Int64 // geohashes fetched from ClickHouse
}

// AddFetchTime records time spent fetching vehicle events.
func (ts *TimingStats) AddFetchTime(d time.Duration) {
	ts.totalFetchTime.Add(d.Nanoseconds())
}

// AddProcessTime records time spent processing events (CPU-bound).
func (ts *TimingStats) AddProcessTime(d time.Duration) {
	ts.totalProcessTime.Add(d.Nanoseconds())
}

// AddSaveTime records time spent saving records to ClickHouse.
func (ts *TimingStats) AddSaveTime(d time.Duration) {
	ts.totalSaveTime.Add(d.Nanoseconds())
}

// RecordLine records a processed line with its event count.
func (ts *TimingStats) RecordLine(eventCount int) {
	ts.linesProcessed.Add(1)
	ts.eventsProcessed.Add(int64(eventCount))
}

// RecordCacheStats records cache hit/miss statistics.
func (ts *TimingStats) RecordCacheStats(hits, misses int) {
	ts.cacheHits.Add(int64(hits))
	ts.cacheMisses.Add(int64(misses))
}

// LogSummary logs a summary of timing statistics.
func (ts *TimingStats) LogSummary() {
	lines := ts.linesProcessed.Load()
	if lines == 0 {
		return
	}

	fetchTotal := time.Duration(ts.totalFetchTime.Load())
	processTotal := time.Duration(ts.totalProcessTime.Load())
	saveTotal := time.Duration(ts.totalSaveTime.Load())
	totalTime := fetchTotal + processTotal + saveTotal
	events := ts.eventsProcessed.Load()

	// Calculate percentages
	fetchPct := float64(fetchTotal) / float64(totalTime) * 100
	processPct := float64(processTotal) / float64(totalTime) * 100
	savePct := float64(saveTotal) / float64(totalTime) * 100

	// Cache statistics
	hits := ts.cacheHits.Load()
	misses := ts.cacheMisses.Load()
	totalGeohashes := hits + misses
	hitRate := float64(0)
	if totalGeohashes > 0 {
		hitRate = float64(hits) / float64(totalGeohashes) * 100
	}

	lib.AppLogger.Info("=== Performance Timing Summary ===")
	lib.AppLogger.Info("  Lines processed: %d, Events processed: %s", lines, lib.FormatCount(int(events)))
	lib.AppLogger.Info("  Fetch (I/O):   %v (%.1f%%)", fetchTotal.Round(time.Millisecond), fetchPct)
	lib.AppLogger.Info("  Process (CPU): %v (%.1f%%)", processTotal.Round(time.Millisecond), processPct)
	lib.AppLogger.Info("  Save (I/O):    %v (%.1f%%)", saveTotal.Round(time.Millisecond), savePct)
	lib.AppLogger.Info("  Avg per line:  fetch=%v, process=%v",
		(fetchTotal / time.Duration(lines)).Round(time.Millisecond),
		(processTotal / time.Duration(lines)).Round(time.Millisecond))
	if events > 0 {
		lib.AppLogger.Info("  Avg per event: %v", (processTotal / time.Duration(events)).Round(time.Microsecond))
	}
	if totalGeohashes > 0 {
		lib.AppLogger.Info("  Cache: %d hits, %d misses (%.1f%% hit rate)", hits, misses, hitRate)
	}
}

// ClickhouseClient defines the ClickHouse operations needed by LineProcessor.
type ClickhouseClient interface {
	FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error)
	FetchVehicleEventsByGeohash(ctx context.Context, geohashes []string, settings *types.Settings) (map[string][]types.VehicleEvent, error)
	DeleteTravelTimesForShapes(ctx context.Context, lineID uint32, hashedShapeIDs []string) error
	SaveTravelTimes(ctx context.Context, records []types.NodeTravelTimeRecord) error
}

// LineProcessor handles parallel processing of lines for travel time calculation.
type LineProcessor struct {
	clickhouse  ClickhouseClient
	settings    *types.Settings
	timingStats *TimingStats
	eventCache  *GeohashEventCache // Cache for vehicle events by geohash
}

// NewLineProcessor creates a new LineProcessor instance.
func NewLineProcessor(ch ClickhouseClient, settings *types.Settings) *LineProcessor {
	return &LineProcessor{
		clickhouse:  ch,
		settings:    settings,
		timingStats: &TimingStats{},
		eventCache:  NewGeohashEventCache(),
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

// ProcessAllLines processes all lines in batches using a worker pool for parallel execution.
// Between batches, it checks for context cancellation to allow graceful shutdown.
// Completed batches are fully persisted to ClickHouse before the next batch starts.
func (lp *LineProcessor) ProcessAllLines(ctx context.Context, lineShapes types.LineShapesMap) error {
	sortedLines := lp.sortLines(lineShapes)
	totalLines := len(sortedLines)

	if totalLines == 0 {
		lib.AppLogger.Info("No lines to process")
		return nil
	}

	batchSize := lp.settings.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	totalBatches := (totalLines + batchSize - 1) / batchSize

	lib.AppLogger.Title(fmt.Sprintf("Processing %d lines with %d workers in batches of %d (%d batches)",
		totalLines, lp.settings.WorkerCount, batchSize, totalBatches))

	// Create a single WorkerDisplay that spans all batches
	display := lib.NewWorkerDisplay(lp.settings.WorkerCount, totalLines)

	var totalRecords, processedLines, skippedLines int
	var firstError error

	for batchIdx := range totalBatches {
		// Check for cancellation between batches
		if ctx.Err() != nil {
			display.Stop()
			lib.AppLogger.Info("Shutdown requested — stopping after batch %d/%d (%d lines processed, %d skipped)",
				batchIdx, totalBatches, processedLines, skippedLines)
			return ctx.Err()
		}

		start := batchIdx * batchSize
		end := min(start + batchSize, totalLines)
		batch := sortedLines[start:end]

		lib.AppLogger.Debug("Starting batch %d/%d (lines %d-%d)",
			batchIdx+1, totalBatches, start+1, end)

		result := lp.processBatch(ctx, batch, display)

		totalRecords += result.records
		processedLines += result.processed
		skippedLines += result.skipped
		if result.err != nil && firstError == nil {
			firstError = result.err
		}

		lib.AppLogger.Debug("Batch %d/%d complete: %d processed, %d skipped, %d records",
			batchIdx+1, totalBatches, result.processed, result.skipped, result.records)
	}

	display.Stop()

	lib.AppLogger.Success(
		"Processing complete: %d lines processed, %d skipped, %d total records saved",
		processedLines, skippedLines, totalRecords,
	)

	// Log timing summary for performance analysis
	lp.timingStats.LogSummary()

	return firstError
}

// batchResult holds the aggregated outcome of processing a single batch.
type batchResult struct {
	processed int
	skipped   int
	records   int
	err       error
}

// processBatch processes a slice of lines using the worker pool pattern.
// Channels are sized to the batch length, bounding memory usage.
// The event cache is cleared at the start of each batch to limit memory usage
// while still allowing cache sharing within a batch.
func (lp *LineProcessor) processBatch(ctx context.Context, batch []sortedLine, display *lib.WorkerDisplay) batchResult {
	// Clear event cache at the start of each batch to limit memory usage
	lp.eventCache.Clear()

	batchLen := len(batch)

	jobs := make(chan lineJob, batchLen)
	results := make(chan lineResult, batchLen)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < lp.settings.WorkerCount; w++ {
		wg.Add(1)
		go lp.worker(ctx, w, jobs, results, &wg, display)
	}

	// Submit batch jobs
	for index, line := range batch {
		jobs <- lineJob{
			index:    index,
			lineID:   line.lineID,
			lineData: line.data,
		}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var br batchResult
	for result := range results {
		if result.err != nil {
			if br.err == nil {
				br.err = result.err
			}
			display.LineErrored()
			lib.AppLogger.Error(result.err, "Failed to process line %d", result.lineID)
			continue
		}

		if result.skipped {
			br.skipped++
			display.LineSkipped()
			continue
		}

		// Save records to ClickHouse (timed)
		if len(result.records) > 0 {
			saveStart := time.Now()
			if err := lp.clickhouse.SaveTravelTimes(ctx, result.records); err != nil {
				lib.AppLogger.Error(err, "Failed to save travel times for line %d", result.lineID)
				if br.err == nil {
					br.err = err
				}
				display.LineErrored()
				continue
			}
			lp.timingStats.AddSaveTime(time.Since(saveStart))
			br.records += len(result.records)
		}

		br.processed++
		display.LineCompleted(len(result.records))
	}

	return br
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

	// Fetch vehicle events for this line's geohashes (timed)
	fetchStart := time.Now()
	vehicleEvents, err := lp.fetchAndPrepareVehicleEvents(ctx, lineID, lineData)
	lp.timingStats.AddFetchTime(time.Since(fetchStart))

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

	// Process the line (timed - CPU-bound)
	processStart := time.Now()
	records := lp.processLine(vehicleEvents, lineID, lineData)
	lp.timingStats.AddProcessTime(time.Since(processStart))

	// Record line statistics
	lp.timingStats.RecordLine(len(vehicleEvents))

	return lineResult{
		index:   job.index,
		lineID:  lineID,
		records: records,
	}
}

// fetchAndPrepareVehicleEvents fetches vehicle events and prepares the line for processing.
// Uses the geohash event cache to avoid re-fetching events for geohashes that were
// already fetched by other lines in the same batch.
func (lp *LineProcessor) fetchAndPrepareVehicleEvents(ctx context.Context, lineID int, lineData *types.LineShapeData) ([]types.VehicleEvent, error) {
	// Convert geohash set to slice
	geohashes := make([]string, 0, len(lineData.Geohashes))
	for gh := range lineData.Geohashes {
		geohashes = append(geohashes, gh)
	}

	// Check which geohashes are already in the cache
	cached, uncached := lp.eventCache.GetCached(geohashes)

	// Record cache statistics
	lp.timingStats.RecordCacheStats(len(cached), len(uncached))

	// Fetch only the uncached geohashes from ClickHouse
	if len(uncached) > 0 {
		freshEvents, err := lp.clickhouse.FetchVehicleEventsByGeohash(ctx, uncached, lp.settings)
		if err != nil {
			return nil, err
		}
		// Add fresh events to cache
		lp.eventCache.SetBatch(freshEvents)
	}

	// Collect all events from cache (both previously cached and freshly fetched)
	allGeohashes := append(cached, uncached...)
	vehicleEvents := lp.eventCache.GetEventsForGeohashes(allGeohashes)

	if len(vehicleEvents) == 0 {
		return nil, nil
	}

	// Delete existing travel time records to prevent duplicates
	if err := lp.clickhouse.DeleteTravelTimesForShapes(ctx, uint32(lineID), lineData.HashedShapeIDs); err != nil {
		return nil, err
	}

	return vehicleEvents, nil
}
