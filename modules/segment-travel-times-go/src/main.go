package main

import (
	"context"
	"fmt"
	"main/src/lib"
	clickhouseService "main/src/services/clickhouse"
	mongoService "main/src/services/mongo"
	"main/src/services/processor"
	"time"
)

func main() {
	// Clear screen and initialize logger
	lib.AppLogger.Clear()
	lib.AppLogger.Init()

	// Load configuration
	config := lib.LoadConfig()
	lib.AppLogger.SetLogLevel(config.LogLevel)

	// Run the main processing loop
	runOnInterval := func() {
		if err := runProcessing(config); err != nil {
			lib.AppLogger.Error(err, "Processing failed")
		}
	}

	// Initial run
	runOnInterval()

	// Schedule subsequent runs at the configured interval
	ticker := time.NewTicker(lib.RunInterval)
	defer ticker.Stop()

	for range ticker.C {
		runOnInterval()
	}
}

// runProcessing executes the main travel time calculation workflow.
func runProcessing(config *lib.Config) error {
	ctx := context.Background()
	globalStart := time.Now()

	lib.AppLogger.Title("Starting travel time calculation")

	// Initialize Clickhouse Client
	clickhouseClient, err := clickhouseService.NewClickhouseClient(config.Clickhouse)
	if err != nil {
		return err
	}
	defer clickhouseClient.Close()

	// Initialize MongoDB Client
	mongoClient, err := mongoService.NewMongoClient(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		return err
	}
	defer mongoClient.Close(ctx)

	// Ensure the travel times table exists
	if err := clickhouseClient.CreateTravelTimesTable(ctx); err != nil {
		return err
	}

	/**
	 * We fetch rides from MongoDB within a data range to know what Shapes were being used in the given period.
	 * This is the node data that's going to be used to calculate the travel times.
	 */
	lib.AppLogger.Info("Fetching rides from MongoDB")
	cursor, totalCount, err := mongoClient.RidesCursor(ctx, config.Settings)
	if err != nil {
		return err
	}
	lib.AppLogger.Info("Found %d rides", totalCount)

	/**
	 * Here we are actually fetching the hashed shapes from the database,
	 * chunking them into nodes of the given length,
	 * and geohashing the coordinates of the endpoints.
	 *
	 * We then group them by their line_id so that we don't fetch the same vehicle events for the same line multiple times.
	 * This could possibly be improved in the future by not fetching the vehicle events for the same geohash more than once,
	 * but for now we assume that the vehicle events in the same line are going to be mostly in the same geohashes.
	 */
	lib.AppLogger.Info("> Aggregating rides to line shapes")
	lineShapes, processedCount, err := mongoClient.AggregateRidesToLineShapes(ctx, cursor, config.Settings, totalCount)
	if err != nil {
		return err
	}
	lib.AppLogger.Info("Grouped %d hashed shapes into %d lines", processedCount, len(lineShapes))

	/**
	 * Process each line to calculate and save travel times.
	 * Uses a worker pool with goroutines for parallel processing.
	 * See processLine() function for detailed documentation on the processing logic.
	 */
	lineProcessor := processor.NewLineProcessor(clickhouseClient, config.Settings)
	if err := lineProcessor.ProcessAllLines(ctx, lineShapes); err != nil {
		lib.AppLogger.Error(err, "Some lines failed to process")
		// Continue to create aggregation tables even if some lines failed
	}

	/**
	 * Create aggregation tables (pivot hourly data, cumulative times, etc.)
	 * These tables are used for efficient ETA queries.
	 */
	if err := clickhouseClient.CreateAggregationTables(ctx); err != nil {
		return err
	}

	lib.AppLogger.Terminate(fmt.Sprintf("Processing completed in %v", time.Since(globalStart)))
	return nil
}