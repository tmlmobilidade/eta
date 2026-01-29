package main

import (
	"context"
	"main/src/lib"
	clickhouseService "main/src/services/clickhouse"
	mongoService "main/src/services/mongo"
)

func main() {
	//

	// Clear screen and initialize logger
	lib.AppLogger.Clear()
	lib.AppLogger.Init()

	// Load configuration
	config := lib.LoadConfig()
	lib.AppLogger.SetLogLevel(config.LogLevel)
	
	// Initialize Clickhouse Client
	clickhouseClient, err := clickhouseService.NewClickhouseClient(config.Clickhouse)
	if err != nil {panic(err)}
	defer clickhouseClient.Close()

	// Initialize MongoDB Client
	mongoClient, err := mongoService.NewMongoClient(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {panic(err)}
	defer mongoClient.Close(context.Background())

	// Ensure the travel times table exists
	if err := clickhouseClient.CreateTravelTimesTable(context.Background()); err != nil {
		panic(err)
	}

	/**
	 * We fetch rides from MongoDB within a data range to know what Shapes were being used in the given period.
	 * This is the node data that's going to be used to calculate the travel times.
	 */
	lib.AppLogger.Info("Fetching rides from MongoDB")
	cursor, totalCount, err := mongoClient.RidesCursor(context.Background(), config.Settings)
	if err != nil {panic(err)}
	lib.AppLogger.Info("Found %d rides", totalCount)

	/**
	 * Here we are actually fetching the hashed shapes from the database,
	 * chunking them into nodes of the given length,
	 * and geohashing the coordinates of the endpoints.
	 *
	 * We then group them by their line_id so that we don't fetch the same vehicle events for the same line multiple times.
	 * This could possibly be improved in the future by not fetching the vehicle events for the same geohashe more than once,
	 * but for now we assume that the vehicle events in the same line are going to be mostly in the same geohashes.
	*/
	lib.AppLogger.Info("> Aggregating rides to line shapes")
	lineShapes, processedCount, err := mongoClient.AggregateRidesToLineShapes(context.Background(), cursor, config.Settings)
	if err != nil {panic(err)}
	lib.AppLogger.Info("Grouped %d hashed shapes into %d lines", processedCount, len(lineShapes))



}