package main

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/processor"
	clickhouseService "main/src/services/clickhouse"
	mongoService "main/src/services/mongo"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func initializeClients(config *lib.Config) (*clickhouseService.ClickhouseClient, *mongoService.MongoClient) {
	//
	
	//	
	// Initialize Clickhouse Client
	
	clickhouseClient, err := clickhouseService.NewClickhouseClient(config.Clickhouse)
	if err != nil {
		panic(err)
	}

	//	
	// Initialize MongoDB Client

	mongoClient, err := mongoService.NewMongoClient(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		panic(err)
	}

	return clickhouseClient, mongoClient
}

func main() {
	// Clear screen and initialize logger
	lib.AppLogger.Clear()
	lib.AppLogger.Init()
	performanceTracker := lib.AppLogger.StartPerformanceTracker("main")

	// Load configuration
	config := lib.LoadConfig()
	lib.AppLogger.SetLogLevel(config.LogLevel)

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//
	// Initialize Clients

	clickhouseClient, mongoClient := initializeClients(config)

	// Ensure ClickHouse tables exist (drop & recreate as needed)
	clickhouseClient.SetupSchema(ctx)
	clickhouseClient.DropAggregationTables(ctx)

	//
	// Fetch unique hashed shapes

	hashedShapesByLine := clickhouseClient.FetchUniqueHashedShapesIDsByLine(ctx)
	lib.AppLogger.Info("Found %d unique hashed shapes", len(hashedShapesByLine))

	//
	// Fetch hashed shapes by IDs

	lineShapesMap := processor.GenerateLineShapes(ctx, mongoClient, &hashedShapesByLine, config.Settings)
	lib.AppLogger.Info("Generated %d line shapes", len(lineShapesMap))

	records := processor.ProcessLineShapes(ctx, clickhouseClient, lineShapesMap, &hashedShapesByLine, config.Settings)
	lib.AppLogger.Info("Generated %d travel time records", len(records))

	clickhouseClient.InsertNodeTravelTimeRecords(ctx, records)

	clickhouseClient.SetupAggregations(ctx)

	duration := performanceTracker.End()
	lib.AppLogger.Terminate(fmt.Sprintf("Segment travel times calculation completed successfully in %v!", duration.Round(time.Second)))
}