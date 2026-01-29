package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"strings"
)

// ============================================================================
// TABLE CREATION FUNCTIONS
// ============================================================================

// generateHourColumns generates column definitions for hours 0-23 with a given prefix and type.
func generateHourColumns(prefix string, dataType string) string {
	columns := make([]string, 24)
	for i := 0; i < 24; i++ {
		columns[i] = fmt.Sprintf("%s%d %s", prefix, i, dataType)
	}
	return strings.Join(columns, ",\n\t\t\t")
}

// generateHourColumnNames generates column names for hours 0-23 with a given prefix.
func generateHourColumnNames(prefix string) string {
	columns := make([]string, 24)
	for i := 0; i < 24; i++ {
		columns[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return strings.Join(columns, ",\n\t\t\t")
}

// CreateTravelTimesHourlyTable creates the hourly pivot table for travel times.
// Columns: hashed_shape_id, node_index, latitude, longitude, h0-h23 (travel times)
// USE CASE: Get travel time for a specific segment at a specific hour.
func (s *ClickhouseService) CreateTravelTimesHourlyTable(ctx context.Context) error {
	hourColumns := generateHourColumns("h", "Float32")

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_travel_times_hourly (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, hourColumns)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create segment_travel_times_hourly table")
	}

	lib.AppLogger.Info("Created segment_travel_times_hourly table")
	return nil
}

// CreateCumulativeTravelTimesTable creates cumulative travel times table (time from start to each node).
// USE CASE: Calculate ETA by subtracting current position from total route time.
// Example: If total route = 1800s and cumulative at node 5 = 300s, remaining = 1500s.
func (s *ClickhouseService) CreateCumulativeTravelTimesTable(ctx context.Context) error {
	hourColumns := generateHourColumns("cumulative_h", "Float32")

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_cumulative_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, hourColumns)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create segment_cumulative_times table")
	}

	lib.AppLogger.Info("Created segment_cumulative_times table")
	return nil
}

// CreateRemainingTimeTable creates remaining time to end table (time from each node to route end).
// USE CASE: Direct ETA lookup - given current node and hour, get remaining time.
// This is the most useful table for real-time ETA predictions.
func (s *ClickhouseService) CreateRemainingTimeTable(ctx context.Context) error {
	hourColumns := generateHourColumns("remaining_h", "Float32")

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_remaining_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, hourColumns)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create segment_remaining_times table")
	}

	lib.AppLogger.Info("Created segment_remaining_times table")
	return nil
}

// CreateTimePeriodTable creates time period aggregation table (rush hour, midday, night).
// USE CASE: Simpler queries when exact hour granularity isn't needed.
// Time periods:
//   - morning_rush: 7-9 (peak congestion)
//   - midday: 10-16 (moderate traffic)
//   - evening_rush: 17-19 (peak congestion)
//   - night: 20-6 (low traffic)
func (s *ClickhouseService) CreateTimePeriodTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS segment_time_periods (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			morning_rush Float32,
			morning_rush_samples UInt32,
			midday Float32,
			midday_samples UInt32,
			evening_rush Float32,
			evening_rush_samples UInt32,
			night Float32,
			night_samples UInt32,
			all_day Float32,
			all_day_samples UInt32
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create segment_time_periods table")
	}

	lib.AppLogger.Info("Created segment_time_periods table")
	return nil
}

// CreateShapeStatisticsTable creates shape statistics table (route-level aggregations).
// USE CASE: Quick route overview, fallback estimates, data quality checks.
// Contains: total nodes, total travel time per period, average speed estimates.
func (s *ClickhouseService) CreateShapeStatisticsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS shape_statistics (
			line_id UInt32,
			hashed_shape_id String,
			total_nodes UInt16,
			total_samples UInt32,
			total_time_morning_rush Float32,
			total_time_midday Float32,
			total_time_evening_rush Float32,
			total_time_night Float32,
			total_time_all_day Float32,
			avg_node_time Float32,
			min_node_time Float32,
			max_node_time Float32
		) ENGINE = ReplacingMergeTree()
		ORDER BY (line_id, hashed_shape_id)
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create shape_statistics table")
	}

	lib.AppLogger.Info("Created shape_statistics table")
	return nil
}

// CreateLineStatisticsTable creates line-level statistics table (fallback when shape data is unavailable).
// USE CASE: Fallback estimates when specific shape has no data.
func (s *ClickhouseService) CreateLineStatisticsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS line_statistics (
			line_id UInt32,
			shape_count UInt16,
			total_samples UInt32,
			avg_time_per_node_morning_rush Float32,
			avg_time_per_node_midday Float32,
			avg_time_per_node_evening_rush Float32,
			avg_time_per_node_night Float32,
			avg_time_per_node_all_day Float32
		) ENGINE = ReplacingMergeTree()
		ORDER BY (line_id)
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create line_statistics table")
	}

	lib.AppLogger.Info("Created line_statistics table")
	return nil
}

// ============================================================================
// POPULATION FUNCTIONS
// ============================================================================

// generateHourAggregations generates avgIf aggregations for hours 0-23.
func generateHourAggregations() string {
	aggregations := make([]string, 24)
	for i := 0; i < 24; i++ {
		aggregations[i] = fmt.Sprintf("avgIf(travel_time_seconds, hour = %d) as h%d", i, i)
	}
	return strings.Join(aggregations, ",\n\t\t\t")
}

// generateCumulativeAggregations generates cumulative sum window functions for hours 0-23.
func generateCumulativeAggregations() string {
	aggregations := make([]string, 24)
	for i := 0; i < 24; i++ {
		aggregations[i] = fmt.Sprintf("sum(h%d) OVER (PARTITION BY hashed_shape_id ORDER BY node_index) as cumulative_h%d", i, i)
	}
	return strings.Join(aggregations, ",\n\t\t\t")
}

// generateTotalColumns generates max window functions for cumulative columns.
func generateTotalColumns() string {
	columns := make([]string, 24)
	for i := 0; i < 24; i++ {
		columns[i] = fmt.Sprintf("max(cumulative_h%d) OVER (PARTITION BY hashed_shape_id) as total_h%d", i, i)
	}
	return strings.Join(columns, ",\n\t\t\t")
}

// generateRemainingColumns generates remaining time calculations.
func generateRemainingColumns() string {
	columns := make([]string, 24)
	for i := 0; i < 24; i++ {
		columns[i] = fmt.Sprintf("total_h%d - cumulative_h%d as remaining_h%d", i, i, i)
	}
	return strings.Join(columns, ",\n\t\t\t")
}

// PopulateTravelTimesHourly populates the hourly travel times pivot table.
func (s *ClickhouseService) PopulateTravelTimesHourly(ctx context.Context) error {
	hourAggregations := generateHourAggregations()
	hourColumnNames := generateHourColumnNames("h")

	query := fmt.Sprintf(`
		INSERT INTO segment_travel_times_hourly (
			hashed_shape_id, node_index, latitude, longitude,
			%s
		)
		SELECT 
			hashed_shape_id,
			node_index,
			any(latitude) as latitude,
			any(longitude) as longitude,
			%s
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
		ORDER BY hashed_shape_id, node_index
	`, hourColumnNames, hourAggregations)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate segment_travel_times_hourly table")
	}

	lib.AppLogger.Info("Populated segment_travel_times_hourly table")
	return nil
}

// PopulateCumulativeTravelTimes populates cumulative travel times (running sum from start to each node).
func (s *ClickhouseService) PopulateCumulativeTravelTimes(ctx context.Context) error {
	cumulativeAggregations := generateCumulativeAggregations()
	cumulativeColumnNames := generateHourColumnNames("cumulative_h")

	query := fmt.Sprintf(`
		INSERT INTO segment_cumulative_times (
			hashed_shape_id, node_index, latitude, longitude,
			%s
		)
		SELECT 
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			%s
		FROM segment_travel_times_hourly
		ORDER BY hashed_shape_id, node_index
	`, cumulativeColumnNames, cumulativeAggregations)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate segment_cumulative_times table")
	}

	lib.AppLogger.Info("Populated segment_cumulative_times table")
	return nil
}

// PopulateRemainingTimes populates remaining time to end (total route time - cumulative time at each node).
func (s *ClickhouseService) PopulateRemainingTimes(ctx context.Context) error {
	remainingColumns := generateRemainingColumns()
	totalColumns := generateTotalColumns()
	remainingColumnNames := generateHourColumnNames("remaining_h")

	// Generate cumulative column names for inner select
	cumulativeColumns := make([]string, 24)
	for i := 0; i < 24; i++ {
		cumulativeColumns[i] = fmt.Sprintf("cumulative_h%d", i)
	}
	cumulativeColumnsList := strings.Join(cumulativeColumns, ", ")

	query := fmt.Sprintf(`
		INSERT INTO segment_remaining_times (
			hashed_shape_id, node_index, latitude, longitude,
			%s
		)
		SELECT 
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			%s
		FROM (
			SELECT 
				hashed_shape_id,
				node_index,
				latitude,
				longitude,
				%s,
				%s
			FROM segment_cumulative_times
		)
		ORDER BY hashed_shape_id, node_index
	`, remainingColumnNames, remainingColumns, cumulativeColumnsList, totalColumns)

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate segment_remaining_times table")
	}

	lib.AppLogger.Info("Populated segment_remaining_times table")
	return nil
}

// PopulateTimePeriods populates time period aggregations.
// Morning rush: hours 7, 8, 9
// Midday: hours 10-16
// Evening rush: hours 17, 18, 19
// Night: hours 20-23, 0-6
func (s *ClickhouseService) PopulateTimePeriods(ctx context.Context) error {
	query := `
		INSERT INTO segment_time_periods (
			hashed_shape_id, node_index, latitude, longitude,
			morning_rush, morning_rush_samples,
			midday, midday_samples,
			evening_rush, evening_rush_samples,
			night, night_samples,
			all_day, all_day_samples
		)
		SELECT 
			hashed_shape_id,
			node_index,
			any(latitude) as latitude,
			any(longitude) as longitude,
			-- Morning rush (7-9)
			avgIf(travel_time_seconds, hour IN (7, 8, 9)) as morning_rush,
			sumIf(sample_count, hour IN (7, 8, 9)) as morning_rush_samples,
			-- Midday (10-16)
			avgIf(travel_time_seconds, hour IN (10, 11, 12, 13, 14, 15, 16)) as midday,
			sumIf(sample_count, hour IN (10, 11, 12, 13, 14, 15, 16)) as midday_samples,
			-- Evening rush (17-19)
			avgIf(travel_time_seconds, hour IN (17, 18, 19)) as evening_rush,
			sumIf(sample_count, hour IN (17, 18, 19)) as evening_rush_samples,
			-- Night (20-23, 0-6)
			avgIf(travel_time_seconds, hour IN (20, 21, 22, 23, 0, 1, 2, 3, 4, 5, 6)) as night,
			sumIf(sample_count, hour IN (20, 21, 22, 23, 0, 1, 2, 3, 4, 5, 6)) as night_samples,
			-- All day average
			avg(travel_time_seconds) as all_day,
			sum(sample_count) as all_day_samples
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
		ORDER BY hashed_shape_id, node_index
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate segment_time_periods table")
	}

	lib.AppLogger.Info("Populated segment_time_periods table")
	return nil
}

// PopulateShapeStatistics populates shape-level statistics.
func (s *ClickhouseService) PopulateShapeStatistics(ctx context.Context) error {
	query := `
		INSERT INTO shape_statistics (
			line_id, hashed_shape_id, total_nodes, total_samples,
			total_time_morning_rush, total_time_midday, total_time_evening_rush, total_time_night, total_time_all_day,
			avg_node_time, min_node_time, max_node_time
		)
		SELECT 
			any(line_id) as line_id,
			hashed_shape_id,
			count() as total_nodes,
			sum(all_day_samples) as total_samples,
			sum(morning_rush) as total_time_morning_rush,
			sum(midday) as total_time_midday,
			sum(evening_rush) as total_time_evening_rush,
			sum(night) as total_time_night,
			sum(all_day) as total_time_all_day,
			avg(all_day) as avg_node_time,
			min(all_day) as min_node_time,
			max(all_day) as max_node_time
		FROM segment_time_periods tp
		LEFT JOIN (
			SELECT DISTINCT hashed_shape_id, line_id 
			FROM segment_travel_times
		) lt USING (hashed_shape_id)
		GROUP BY hashed_shape_id
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate shape_statistics table")
	}

	lib.AppLogger.Info("Populated shape_statistics table")
	return nil
}

// PopulateLineStatistics populates line-level statistics (averages across all shapes in a line).
func (s *ClickhouseService) PopulateLineStatistics(ctx context.Context) error {
	query := `
		INSERT INTO line_statistics (
			line_id, shape_count, total_samples,
			avg_time_per_node_morning_rush, avg_time_per_node_midday,
			avg_time_per_node_evening_rush, avg_time_per_node_night, avg_time_per_node_all_day
		)
		SELECT 
			line_id,
			uniqExact(hashed_shape_id) as shape_count,
			sum(total_samples) as total_samples,
			avgIf(avg_node_time, total_time_morning_rush > 0) as avg_time_per_node_morning_rush,
			avgIf(avg_node_time, total_time_midday > 0) as avg_time_per_node_midday,
			avgIf(avg_node_time, total_time_evening_rush > 0) as avg_time_per_node_evening_rush,
			avgIf(avg_node_time, total_time_night > 0) as avg_time_per_node_night,
			avg(avg_node_time) as avg_time_per_node_all_day
		FROM shape_statistics
		GROUP BY line_id
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to populate line_statistics table")
	}

	lib.AppLogger.Info("Populated line_statistics table")
	return nil
}

// ============================================================================
// MANAGEMENT FUNCTIONS
// ============================================================================

// aggregationTables lists all aggregation table names for drop/create operations.
var aggregationTables = []string{
	"segment_travel_times_hourly",
	"segment_cumulative_times",
	"segment_remaining_times",
	"segment_time_periods",
	"shape_statistics",
	"line_statistics",
}

// DropAggregationTables drops all aggregation tables.
func (s *ClickhouseService) DropAggregationTables(ctx context.Context) error {
	for _, table := range aggregationTables {
		query := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
		if err := s.conn.Exec(ctx, query); err != nil {
			return lib.AppLogger.Error(err, "failed to drop table %s", table)
		}
	}

	lib.AppLogger.Info("Dropped existing aggregation tables")
	return nil
}

// CreateAggregationTables creates and populates all aggregation tables.
// Tables are created in dependency order (some tables depend on others).
func (s *ClickhouseService) CreateAggregationTables(ctx context.Context) error {
	lib.AppLogger.Title("Creating ETA aggregation tables")

	// Drop existing tables to ensure correct schema
	if err := s.DropAggregationTables(ctx); err != nil {
		return err
	}

	// Create all tables
	if err := s.CreateTravelTimesHourlyTable(ctx); err != nil {
		return err
	}
	if err := s.CreateCumulativeTravelTimesTable(ctx); err != nil {
		return err
	}
	if err := s.CreateRemainingTimeTable(ctx); err != nil {
		return err
	}
	if err := s.CreateTimePeriodTable(ctx); err != nil {
		return err
	}
	if err := s.CreateShapeStatisticsTable(ctx); err != nil {
		return err
	}
	if err := s.CreateLineStatisticsTable(ctx); err != nil {
		return err
	}

	// Populate in dependency order
	// 1. Base hourly pivot (no dependencies)
	if err := s.PopulateTravelTimesHourly(ctx); err != nil {
		return err
	}

	// 2. Cumulative times (depends on hourly)
	if err := s.PopulateCumulativeTravelTimes(ctx); err != nil {
		return err
	}

	// 3. Remaining times (depends on cumulative)
	if err := s.PopulateRemainingTimes(ctx); err != nil {
		return err
	}

	// 4. Time periods (no dependencies, from base table)
	if err := s.PopulateTimePeriods(ctx); err != nil {
		return err
	}

	// 5. Shape statistics (depends on time periods)
	if err := s.PopulateShapeStatistics(ctx); err != nil {
		return err
	}

	// 6. Line statistics (depends on shape statistics)
	if err := s.PopulateLineStatistics(ctx); err != nil {
		return err
	}

	lib.AppLogger.Success("ETA aggregation tables created and populated")
	return nil
}
