# Segment Travel Times - Go Implementation

High-performance Go implementation of the segment travel times processor, leveraging goroutines for parallel processing.

## Features

- **Line-level parallelism**: Process multiple bus lines concurrently using a configurable worker pool
- **Shape-level parallelism**: Process multiple shapes within each line concurrently
- **Direct database access**: Native ClickHouse and MongoDB clients
- **Same functionality**: Produces identical results to the TypeScript implementation

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Main Orchestration                          │
│  Fetch Rides (MongoDB) → Aggregate to LineShapes → Worker Pool     │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
              ┌──────────┐   ┌──────────┐   ┌──────────┐
              │ Worker 1 │   │ Worker 2 │   │ Worker N │
              └────┬─────┘   └────┬─────┘   └────┬─────┘
                   │              │              │
                   ▼              ▼              ▼
         ┌─────────────────────────────────────────────┐
         │            Per-Line Processing              │
         │  Fetch Events → Process Shapes → Save       │
         └─────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ Shape 1  │   │ Shape 2  │   │ Shape N  │
        │ goroutine│   │ goroutine│   │ goroutine│
        └──────────┘   └──────────┘   └──────────┘
```

## Prerequisites

- Go 1.22 or later
- Access to ClickHouse database with `vehicle_events` table
- Access to MongoDB database with `rides` and `hashedshapes` collections

## Installation

```bash
# Clone the repository
cd modules/segment-travel-times-go

# Download dependencies
make deps

# Build
make build
```

## Configuration

Configuration is loaded from environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICKHOUSE_HOST` | `localhost` | ClickHouse server host |
| `CLICKHOUSE_PORT` | `9000` | ClickHouse server port |
| `CLICKHOUSE_DATABASE` | `default` | ClickHouse database name |
| `CLICKHOUSE_USERNAME` | `default` | ClickHouse username |
| `CLICKHOUSE_PASSWORD` | `` | ClickHouse password |
| `CLICKHOUSE_URL` | `` | ClickHouse DSN (overrides individual settings) |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection URI |
| `MONGODB_DATABASE` | `tml` | MongoDB database name |
| `WORKER_COUNT` | `NumCPU()` (max 8) | Number of parallel line workers |
| `BEARING_THRESHOLD` | `90.0` | Bearing threshold in degrees |
| `GEOHASH_PRECISION` | `7` | Geohash precision level |
| `SEGMENT_LENGTH_METERS` | `50.0` | Segment length in meters |

## Usage

```bash
# Run with default configuration
make run

# Run in development mode (no build step)
make dev

# Run with custom worker count
WORKER_COUNT=4 make run
```

## Project Structure

```
segment-travel-times-go/
├── cmd/
│   └── processor/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration loading
│   ├── clickhouse/
│   │   ├── client.go            # ClickHouse connection
│   │   ├── storage.go           # Table creation, save/delete
│   │   ├── queries.go           # Vehicle events queries
│   │   └── aggregations.go      # Aggregation table management
│   ├── mongodb/
│   │   ├── client.go            # MongoDB connection
│   │   ├── rides.go             # Rides service
│   │   └── shapes.go            # Hashed shapes service
│   ├── processor/
│   │   ├── line_processor.go    # Line processing with worker pool
│   │   ├── travel_time.go       # Travel time calculations
│   │   └── aggregator.go        # Rides to line shapes aggregation
│   ├── geo/
│   │   ├── bearing.go           # Bearing calculations
│   │   ├── distance.go          # Haversine distance
│   │   └── geohash.go           # Geohash encoding
│   └── types/
│       └── types.go             # Type definitions
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Performance

The Go implementation provides significant performance improvements over the TypeScript version:

1. **Parallel line processing**: Multiple lines are processed concurrently by the worker pool
2. **Parallel shape processing**: Within each line, shapes are processed concurrently
3. **Native code**: No JavaScript runtime overhead
4. **Efficient memory usage**: Go's garbage collector and memory model

Typical speedup: 3-5x depending on workload and hardware.

## Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage
```

## Comparison with TypeScript

| Feature | TypeScript | Go |
|---------|------------|-----|
| Line processing | Sequential | Parallel (worker pool) |
| Shape processing | Sequential | Parallel (goroutines) |
| ClickHouse client | @clickhouse/client | clickhouse-go/v2 |
| MongoDB client | @tmlmobilidade/interfaces | mongo-driver |
| Geo calculations | @turf/turf | Native implementation |

## License

See LICENSE file in repository root.
