package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"strconv"
	"strings"

	_ "embed"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// *************
// *  Queries  *
// *************

//go:embed queries/fetch-vehicle-events.sql
var fetchVehicleEventsQuery string

//go:embed queries/unique-hashed-shapes-by-line.sql
var uniqueHashedShapesByLineQuery string

//go:embed queries/drop-node-travel-times.sql
var dropNodeTravelTimesTableQuery string

//go:embed queries/create-node-travel-times.sql
var createNodeTravelTimesTableQuery string

//go:embed queries/create-node-travel-times-samples.sql
var createNodeTravelTimesSamplesTableQuery string

//go:embed queries/create-shape-nodes.sql
var createShapeNodesTableQuery string

//go:embed queries/trasnformation-pipeline.sql
var transformationPipelineQuery string

//go:embed queries/create-shape-hourly-summary.sql
var createShapeHourlySummaryQuery string

//go:embed queries/populate-shape-hourly-summary.sql
var populateShapeHourlySummaryQuery string

//go:embed queries/create-hourly-network-summary.sql
var createHourlyNetworkSummaryQuery string

//go:embed queries/populate-hourly-network-summary.sql
var populateHourlyNetworkSummaryQuery string

//go:embed queries/create-node-congestion-analysis.sql
var createNodeCongestionAnalysisQuery string

//go:embed queries/populate-node-congestion-analysis.sql
var populateNodeCongestionAnalysisQuery string

//go:embed queries/create-shape-performance.sql
var createShapePerformanceQuery string

//go:embed queries/populate-shape-performance.sql
var populateShapePerformanceQuery string

// *************
// * Functions *
// *************

/**
	Fetches Vehicle Events from filtered by geolocation.
	Returns deduplicated vehicle events by ride_id and created_at.
	Only returns trips with sufficient data points (min_events).

	@param ctx context.Context: The context for the request.
	@param geohashes []string: The geohashes to filter the vehicle events by.
	@param settings *types.Settings: The settings for the request (min_events).
	@return []types.VehicleEvent: The vehicle events.
	@return error: The error if the request fails.
*/
func (c *ClickhouseClient) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) []types.VehicleEvent {
	if len(geohashes) == 0 {
		return nil
	}

	// Build the IN clause with quoted strings
	quotedGeohashes := make([]string, len(geohashes))
	for i, gh := range geohashes {
		quotedGeohashes[i] = fmt.Sprintf("'%s'", gh)
	}

	// Scanner function to scan the vehicle events
	scanner := func(rows driver.Rows) (types.VehicleEvent, error) {
		var event types.VehicleEvent
		if err := rows.ScanStruct(&event); err != nil {
			return types.VehicleEvent{}, err
		}
		return event, nil
	}

	query := strings.ReplaceAll(fetchVehicleEventsQuery, "{geohashes}", strings.Join(quotedGeohashes, ","))
	query = strings.ReplaceAll(query, "{min_events}", strconv.Itoa(settings.MinEvents))
	
	events, err := QueryAll(c, ctx, query, scanner);
	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to fetch vehicle events"))
	}

	return events
}

// InsertShapeNodes inserts a single shape node record into the shape_nodes table.
func (c *ClickhouseClient) InsertShapeNodes(ctx context.Context, nodes []types.ShapeNode) {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO shape_nodes")
	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to prepare batch for shape_nodes"))
	}

	for _, node := range nodes {
		if err := batch.AppendStruct(&node); err != nil {
			panic(lib.AppLogger.Error(err, "failed to append shape node to batch"))
		}
	}

	if err := batch.Send(); err != nil {
		panic(lib.AppLogger.Error(err, "failed to send batch to shape_nodes"))
	}
}

/**
	Fetches unique hashed shapes from the database.
	@return []string: The unique hashed shapes.
	@return error: The error if the request fails.
*/
func (c *ClickhouseClient) InsertNodeTravelTimeRecords(ctx context.Context, records []types.NodeTravelTimeRecord) {
	if len(records) == 0 {
		return
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO node_travel_times")
	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to prepare batch for node_travel_times"))
	}

	for i := range records {
		if err := batch.AppendStruct(&records[i]); err != nil {
			panic(lib.AppLogger.Error(err, "failed to append record to batch"))
		}
	}

	if err := batch.Send(); err != nil {
		panic(lib.AppLogger.Error(err, "failed to send batch to node_travel_times"))
	}

	lib.AppLogger.Info("Inserted %d records into node_travel_times", len(records))
}

// RunTransformationPipeline executes the ClickHouse SQL pipeline that
// populates node_travel_times_samples from vehicle_events and shape_nodes.
func (c *ClickhouseClient) RunTransformationPipeline(ctx context.Context) {
	if err := c.conn.Exec(ctx, transformationPipelineQuery); err != nil {
		panic(lib.AppLogger.Error(err, "failed to execute transformation pipeline"))
	}
	lib.AppLogger.Info("Transformation pipeline executed successfully")
}

// SetupSchema drops and recreates tables used by this service.
// Currently manages the node_travel_times table.
func (c *ClickhouseClient) SetupSchema(ctx context.Context) {
	// Drop tables if they exist
	if err := c.conn.Exec(ctx, dropNodeTravelTimesTableQuery); err != nil {
		panic(lib.AppLogger.Error(err, "failed to drop node_travel_times table"))
	}

	if err := c.conn.Exec(ctx, "DROP TABLE IF EXISTS shape_nodes"); err != nil {
		panic(lib.AppLogger.Error(err, "failed to drop shape_nodes table"))
	}

	if err := c.conn.Exec(ctx, "DROP TABLE IF EXISTS node_travel_times_samples"); err != nil {
		panic(lib.AppLogger.Error(err, "failed to drop node_travel_times_samples table"))
	}

	// Create tables
	if err := c.conn.Exec(ctx, createNodeTravelTimesTableQuery); err != nil {
		panic(lib.AppLogger.Error(err, "failed to create node_travel_times table"))
	}

	if err := c.conn.Exec(ctx, createShapeNodesTableQuery); err != nil {
		panic(lib.AppLogger.Error(err, "failed to create shape_nodes table"))
	}

	if err := c.conn.Exec(ctx, createNodeTravelTimesSamplesTableQuery); err != nil {
		panic(lib.AppLogger.Error(err, "failed to create node_travel_times_samples table"))
	}

	lib.AppLogger.Info("Schema setup completed for node_travel_times, shape_nodes, and node_travel_times_samples tables")
}

type aggregationTable struct {
	name     string
	create   string
	populate string
}

var aggregationTables = []string{
	"shape_hourly_summary",
	"hourly_network_summary",
	"node_congestion_analysis",
	"shape_performance",
}

// SetupAggregations creates, and populates all aggregation tables
// derived from node_travel_times. Must be called after data insertion.
// Order matters: shape_performance depends on shape_hourly_summary.
func (c *ClickhouseClient) SetupAggregations(ctx context.Context) {
	tables := []aggregationTable{
		{name: "shape_hourly_summary", create: createShapeHourlySummaryQuery, populate: populateShapeHourlySummaryQuery},
		{name: "hourly_network_summary", create: createHourlyNetworkSummaryQuery, populate: populateHourlyNetworkSummaryQuery},
		{name: "node_congestion_analysis", create: createNodeCongestionAnalysisQuery, populate: populateNodeCongestionAnalysisQuery},
		{name: "shape_performance", create: createShapePerformanceQuery, populate: populateShapePerformanceQuery},
	}

	for _, t := range tables {
		if err := c.conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", t.name)); err != nil {
			panic(lib.AppLogger.Error(err, "failed to drop %s table", t.name))
		}
		if err := c.conn.Exec(ctx, t.create); err != nil {
			panic(lib.AppLogger.Error(err, "failed to create %s table", t.name))
		}
		if err := c.conn.Exec(ctx, t.populate); err != nil {
			panic(lib.AppLogger.Error(err, "failed to populate %s table", t.name))
		}
		lib.AppLogger.Info("Aggregation table %s created and populated", t.name)
	}
}


// DropAggregationTables drops all aggregation tables.
func (c *ClickhouseClient) DropAggregationTables(ctx context.Context) {
	for _, t := range aggregationTables {
		if err := c.conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", t)); err != nil {
			panic(lib.AppLogger.Error(err, "failed to drop %s table", t))
		}
		lib.AppLogger.Info("Aggregation table %s dropped", t)
	}
}

func (c *ClickhouseClient) FetchUniqueHashedShapesIDsByLine(ctx context.Context) types.HashedShapeIdsByLineArray {

	// Scanner function to scan the unique hashed shapes
	scanner := func(rows driver.Rows) (types.HashedShapeIdsByLine, error) {
		var hashedShapeIdsByLine types.HashedShapeIdsByLine
		if err := rows.ScanStruct(&hashedShapeIdsByLine); err != nil {
			return types.HashedShapeIdsByLine{}, err
		}
		return hashedShapeIdsByLine, nil
	}
	
	hashedShapeIdsByLine, err := QueryAll(c, ctx, uniqueHashedShapesByLineQuery, scanner);

	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to fetch unique hashed shapes"))
	}

	
	return hashedShapeIdsByLine
}