package main

import (
	"context"
	"main/src/lib"
	clickhouseService "main/src/services/clickhouse"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Clear screen and initialize logger
	lib.AppLogger.Clear()
	lib.AppLogger.Init()

	// Load configuration
	config := lib.LoadConfig()
	lib.AppLogger.SetLogLevel(config.LogLevel)

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	globalStart := time.Now()

	lib.AppLogger.Title("Starting travel time calculation")

	// Initialize Clickhouse Client
	clickhouseClient, err := clickhouseService.NewClickhouseClient(config.Clickhouse)
	if err != nil {
		lib.AppLogger.Error(err, "failed to initialize Clickhouse Client")
		return
	}
	defer clickhouseClient.Close()

	// Fetch unique hashed shapes
	uniqueHashedShapes, err := clickhouseClient.FetchUniqueHashedShapes(ctx)
	if err != nil {
		lib.AppLogger.Error(err, "failed to fetch unique hashed shapes")
		return
	}
	lib.AppLogger.Info("Fetched %d (%s) unique hashed shapes in %s", len(uniqueHashedShapes), strings.Join(uniqueHashedShapes, ", "), time.Since(globalStart))
}