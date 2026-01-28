// Package main is the entry point for the segment travel times processor.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/clickhouse"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/config"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/mongodb"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/processor"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting segment travel times processor")

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Run the main loop
	runOnInterval(ctx)
}

func runOnInterval(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, exiting")
			return
		default:
		}

		if err := run(ctx); err != nil {
			log.Printf("Error in processing run: %v", err)
		}

		log.Printf("Waiting %v until next run...", config.RunInterval)

		select {
		case <-ctx.Done():
			log.Println("Context cancelled during wait, exiting")
			return
		case <-time.After(config.RunInterval):
			// Continue to next iteration
		}
	}
}

func run(ctx context.Context) error {
	globalStart := time.Now()
	log.Println("========================================")
	log.Println("Starting processing run")
	log.Println("========================================")

	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("Configuration loaded:")
	log.Printf("  - Bearing threshold: %.1f degrees", cfg.Settings.BearingThreshold)
	log.Printf("  - Geohash precision: %d", cfg.Settings.GeohashPrecision)
	log.Printf("  - Segment length: %.1f meters", cfg.Settings.SegmentLengthMeters)
	log.Printf("  - Worker count: %d", cfg.Settings.WorkerCount)
	log.Printf("  - Date range: %s to %s",
		time.UnixMilli(cfg.Settings.RideStartDate).Format("2006-01-02 15:04:05"),
		time.UnixMilli(cfg.Settings.RideEndDate).Format("2006-01-02 15:04:05"))

	// Connect to ClickHouse
	var chClient *clickhouse.Client
	var err error

	if cfg.ClickHouseURL != "" {
		chClient, err = clickhouse.NewClientFromURL(cfg.ClickHouseURL)
	} else {
		chClient, err = clickhouse.NewClient(
			cfg.ClickHouseHost,
			cfg.ClickHousePort,
			cfg.ClickHouseDatabase,
			cfg.ClickHouseUsername,
			cfg.ClickHousePassword,
		)
	}
	if err != nil {
		return err
	}
	defer chClient.Close()
	log.Println("Connected to ClickHouse")

	// Connect to MongoDB
	mongoClient, err := mongodb.NewClient(cfg.MongoDBURI, cfg.MongoDBDatabase)
	if err != nil {
		return err
	}
	defer mongoClient.Close(ctx)
	log.Println("Connected to MongoDB")

	// Ensure the travel times table exists
	if err := chClient.CreateTravelTimesTable(ctx); err != nil {
		return err
	}

	// Create services
	ridesService := mongodb.NewRidesService(mongoClient)
	shapesService := mongodb.NewShapesService(mongoClient)

	// Aggregate rides to line shapes
	log.Println("----------------------------------------")
	log.Println("Aggregating rides to line shapes...")
	aggregator := processor.NewAggregator(ridesService, shapesService)
	lineShapes, processedCount, err := aggregator.AggregateRidesToLineShapes(ctx, cfg.Settings)
	if err != nil {
		return err
	}
	log.Printf("Aggregated %d hashed shapes into %d lines", processedCount, len(lineShapes))

	// Process all lines
	log.Println("----------------------------------------")
	log.Println("Processing lines...")
	lineProcessor := processor.NewLineProcessor(chClient, cfg.Settings)
	if err := lineProcessor.ProcessAllLines(ctx, lineShapes); err != nil {
		return err
	}

	// Create aggregation tables
	log.Println("----------------------------------------")
	log.Println("Creating aggregation tables...")
	if err := chClient.CreateAggregationTables(ctx); err != nil {
		return err
	}

	log.Println("========================================")
	log.Printf("Processing run completed in %v", time.Since(globalStart))
	log.Println("========================================")

	return nil
}
