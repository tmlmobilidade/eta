package lib

import (
	"flag"
	"os"
	"runtime"
	"strconv"
	"time"

	"main/src/types"

	"github.com/joho/godotenv"
)

// RunInterval is the interval between processing runs (10 minutes).
const RunInterval = 10 * time.Minute

type Flags struct {
	EnvFile string
	LogLevel string
}

type MongoDBConfig struct {
	URI string
	Database string
}

// Config holds all configuration for the application.
type Config struct {
	// ClickHouse configuration
	Clickhouse types.ClickhouseClientParams

	// MongoDB configuration
	MongoDB MongoDBConfig

	// Processing settings
	Settings *types.Settings

	// Logging configuration
	LogLevel string
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the integer value of an environment variable or a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat returns the float value of an environment variable or a default value.
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func loadFlags() *Flags {
	flags := &Flags{}

	flag.StringVar(&flags.EnvFile, "env", ".env", "Environment variable file")
	flag.StringVar(&flags.LogLevel, "log", "info", "Logging level")

	flag.Parse()

	return flags
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() *Config {
	// Load Environment Variable File
	flags := loadFlags()
	if flags.EnvFile != "" {
		err := godotenv.Load(flags.EnvFile)
		if err != nil {
			AppLogger.Fatalf("Error loading environment variables from file %s: %v", flags.EnvFile, err.Error())
		}
	}

	// Calculate date range (last 7 days, starting at 4 AM Lisbon time)
	location, _ := time.LoadLocation("Europe/Lisbon")
	now := time.Now().In(location)

	// Set to 4:00 AM today
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, location)
	// If current time is before 4 AM, use yesterday's 4 AM
	if now.Before(endDate) {
		endDate = endDate.AddDate(0, 0, -1)
	}

	// Start date is 7 days before end date
	startDate := endDate.AddDate(0, 0, -7)

	// Default worker count is the number of CPUs
	defaultWorkers := min(runtime.NumCPU(), 8)

	return &Config{

		Clickhouse: types.ClickhouseClientParams{
			Host:     getEnv("CLICKHOUSE_HOST", "localhost"),
			Port:     getEnvInt("CLICKHOUSE_PORT", 9000),
			Database: getEnv("CLICKHOUSE_DATABASE", "default"),
			Username: getEnv("CLICKHOUSE_USERNAME", "default"),
			Password: getEnv("CLICKHOUSE_PASSWORD", ""),
		},

		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://localhost:27017"),
			Database: getEnv("MONGODB_DATABASE", "production"),
		},

		Settings: &types.Settings{
			BearingThreshold:           getEnvFloat("BEARING_THRESHOLD", 90.0),
			GeohashPrecision:           getEnvInt("GEOHASH_PRECISION", 7),
			RideEndDate:                endDate.UnixMilli(),
			RideStartDate:              startDate.UnixMilli(),
			SegmentLengthMeters:        getEnvFloat("SEGMENT_LENGTH_METERS", 25.0),
			MaxNodeMatchDistanceMeters: getEnvFloat("MAX_NODE_MATCH_DISTANCE_METERS", 30.0),
			WorkerCount:                getEnvInt("WORKER_COUNT", defaultWorkers),
			BatchSize:                  getEnvInt("BATCH_SIZE", 50),
		},

		LogLevel: flags.LogLevel,
	}
}