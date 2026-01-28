package processor

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/clickhouse"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

// LineProcessor handles the processing of lines with parallel workers.
type LineProcessor struct {
	chClient *clickhouse.Client
	settings *types.Settings
}

// NewLineProcessor creates a new LineProcessor.
func NewLineProcessor(chClient *clickhouse.Client, settings *types.Settings) *LineProcessor {
	return &LineProcessor{
		chClient: chClient,
		settings: settings,
	}
}

// fetchAndPrepareVehicleEvents fetches vehicle events and prepares the line for processing.
func (p *LineProcessor) fetchAndPrepareVehicleEvents(
	ctx context.Context,
	geohashes []string,
	lineID int,
	hashedShapeIDs []string,
) ([]types.VehicleEvent, error) {
	// Fetch vehicle events
	vehicleEvents, err := p.chClient.FetchVehicleEvents(ctx, geohashes, p.settings)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vehicle events: %w", err)
	}

	if len(vehicleEvents) == 0 {
		return nil, nil
	}

	// Delete existing travel time records for this line's shapes to prevent duplicates
	if err := p.chClient.DeleteTravelTimesForShapes(ctx, lineID, hashedShapeIDs); err != nil {
		return nil, fmt.Errorf("failed to delete existing records: %w", err)
	}

	return vehicleEvents, nil
}

// processLine processes a single line and returns travel time records.
func (p *LineProcessor) processLine(
	ctx context.Context,
	lineID int,
	lineData *types.LineShapeData,
) ([]types.NodeTravelTimeRecord, error) {
	// Convert geohashes set to slice
	geohashes := make([]string, 0, len(lineData.Geohashes))
	for gh := range lineData.Geohashes {
		geohashes = append(geohashes, gh)
	}

	// Fetch and prepare vehicle events
	vehicleEvents, err := p.fetchAndPrepareVehicleEvents(ctx, geohashes, lineID, lineData.HashedShapeIDs)
	if err != nil {
		return nil, err
	}

	if vehicleEvents == nil {
		return nil, nil // No events found
	}

	log.Printf("Found %d vehicle events for line %d", len(vehicleEvents), lineID)

	// Process shapes concurrently and calculate travel times
	records := ProcessShapeTravelTimesConcurrent(
		vehicleEvents,
		lineData,
		lineID,
		p.settings.BearingThreshold,
	)

	return records, nil
}

// ProcessAllLines processes all lines using a worker pool for parallel execution.
func (p *LineProcessor) ProcessAllLines(ctx context.Context, lineShapes map[int]*types.LineShapeData) error {
	// Sort lines by ID for consistent ordering
	sortedLines := make([]types.LineEntry, 0, len(lineShapes))
	for lineID, data := range lineShapes {
		sortedLines = append(sortedLines, types.LineEntry{LineID: lineID, Data: data})
	}
	sort.Slice(sortedLines, func(i, j int) bool {
		return sortedLines[i].LineID < sortedLines[j].LineID
	})

	totalLines := len(sortedLines)
	log.Printf("Processing %d lines with %d workers", totalLines, p.settings.WorkerCount)

	// Create channels
	linesChan := make(chan types.LineEntry, totalLines)
	errChan := make(chan error, p.settings.WorkerCount)
	var processedCount int64

	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < p.settings.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for line := range linesChan {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				current := atomic.AddInt64(&processedCount, 1)
				start := time.Now()

				log.Printf("[%d/%d] Worker %d: Processing line %d with %d shapes and %d geohashes",
					current, totalLines, workerID, line.LineID,
					len(line.Data.HashedShapeIDs), len(line.Data.Geohashes))

				// Process the line
				records, err := p.processLine(ctx, line.LineID, line.Data)
				if err != nil {
					log.Printf("Error processing line %d: %v", line.LineID, err)
					errChan <- fmt.Errorf("line %d: %w", line.LineID, err)
					return
				}

				// Save records if any
				if len(records) > 0 {
					if err := p.chClient.SaveTravelTimes(ctx, records); err != nil {
						log.Printf("Error saving records for line %d: %v", line.LineID, err)
						errChan <- fmt.Errorf("save line %d: %w", line.LineID, err)
						return
					}
					log.Printf("[%d/%d] Worker %d: Saved %d records for line %d in %v",
						current, totalLines, workerID, len(records), line.LineID, time.Since(start))
				} else {
					log.Printf("[%d/%d] Worker %d: No records generated for line %d in %v",
						current, totalLines, workerID, line.LineID, time.Since(start))
				}
			}
		}(i)
	}

	// Feed lines to workers
	for _, line := range sortedLines {
		linesChan <- line
	}
	close(linesChan)

	// Wait for workers to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	log.Printf("Successfully processed all %d lines", totalLines)
	return nil
}

// ProcessAllLinesSequential processes all lines sequentially (for debugging/comparison).
func (p *LineProcessor) ProcessAllLinesSequential(ctx context.Context, lineShapes map[int]*types.LineShapeData) error {
	// Sort lines by ID for consistent ordering
	sortedLines := make([]types.LineEntry, 0, len(lineShapes))
	for lineID, data := range lineShapes {
		sortedLines = append(sortedLines, types.LineEntry{LineID: lineID, Data: data})
	}
	sort.Slice(sortedLines, func(i, j int) bool {
		return sortedLines[i].LineID < sortedLines[j].LineID
	})

	totalLines := len(sortedLines)

	for i, line := range sortedLines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()

		log.Printf("[%d/%d] Processing line %d with %d shapes and %d geohashes",
			i+1, totalLines, line.LineID,
			len(line.Data.HashedShapeIDs), len(line.Data.Geohashes))

		// Process the line
		records, err := p.processLine(ctx, line.LineID, line.Data)
		if err != nil {
			return fmt.Errorf("line %d: %w", line.LineID, err)
		}

		log.Printf("Processed %d travel time records in %v", len(records), time.Since(start))

		// Save records if any
		if len(records) > 0 {
			if err := p.chClient.SaveTravelTimes(ctx, records); err != nil {
				return fmt.Errorf("save line %d: %w", line.LineID, err)
			}
			log.Printf("Saved %d records for line %d", len(records), line.LineID)
		} else {
			log.Printf("No records generated for line %d", line.LineID)
		}

		log.Println("----------------------------------------")
	}

	return nil
}
