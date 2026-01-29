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
	
}