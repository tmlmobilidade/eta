package clickhouse

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// CreateAggregationTables creates and populates all aggregation tables.
// Tables are created in dependency order.
func (c *Client) CreateAggregationTables(ctx context.Context) error {
	log.Println("Creating ETA aggregation tables")

	// Drop existing tables
	if err := c.dropAggregationTables(ctx); err != nil {
		return err
	}

	// Create all tables
	if err := c.createTravelTimesHourlyTable(ctx); err != nil {
		return err
	}
	if err := c.createCumulativeTravelTimesTable(ctx); err != nil {
		return err
	}
	if err := c.createRemainingTimeTable(ctx); err != nil {
		return err
	}
	if err := c.createTimePeriodTable(ctx); err != nil {
		return err
	}
	if err := c.createShapeStatisticsTable(ctx); err != nil {
		return err
	}
	if err := c.createLineStatisticsTable(ctx); err != nil {
		return err
	}

	// Populate in dependency order
	if err := c.populateTravelTimesHourly(ctx); err != nil {
		return err
	}
	if err := c.populateCumulativeTravelTimes(ctx); err != nil {
		return err
	}
	if err := c.populateRemainingTimes(ctx); err != nil {
		return err
	}
	if err := c.populateTimePeriods(ctx); err != nil {
		return err
	}
	if err := c.populateShapeStatistics(ctx); err != nil {
		return err
	}
	if err := c.populateLineStatistics(ctx); err != nil {
		return err
	}

	log.Println("ETA aggregation tables created and populated")
	return nil
}

func (c *Client) dropAggregationTables(ctx context.Context) error {
	tables := []string{
		"segment_travel_times_hourly",
		"segment_cumulative_times",
		"segment_remaining_times",
		"segment_time_periods",
		"shape_statistics",
		"line_statistics",
	}

	for _, table := range tables {
		if err := c.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	log.Println("Dropped existing aggregation tables")
	return nil
}

func (c *Client) createTravelTimesHourlyTable(ctx context.Context) error {
	hourColumns := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourColumns[i] = fmt.Sprintf("h%d Float32", i)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_travel_times_hourly (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, strings.Join(hourColumns, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create segment_travel_times_hourly: %w", err)
	}
	log.Println("Created segment_travel_times_hourly table")
	return nil
}

func (c *Client) createCumulativeTravelTimesTable(ctx context.Context) error {
	hourColumns := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourColumns[i] = fmt.Sprintf("cumulative_h%d Float32", i)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_cumulative_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, strings.Join(hourColumns, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create segment_cumulative_times: %w", err)
	}
	log.Println("Created segment_cumulative_times table")
	return nil
}

func (c *Client) createRemainingTimeTable(ctx context.Context) error {
	hourColumns := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourColumns[i] = fmt.Sprintf("remaining_h%d Float32", i)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS segment_remaining_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			%s
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`, strings.Join(hourColumns, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create segment_remaining_times: %w", err)
	}
	log.Println("Created segment_remaining_times table")
	return nil
}

func (c *Client) createTimePeriodTable(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create segment_time_periods: %w", err)
	}
	log.Println("Created segment_time_periods table")
	return nil
}

func (c *Client) createShapeStatisticsTable(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create shape_statistics: %w", err)
	}
	log.Println("Created shape_statistics table")
	return nil
}

func (c *Client) createLineStatisticsTable(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create line_statistics: %w", err)
	}
	log.Println("Created line_statistics table")
	return nil
}

func (c *Client) populateTravelTimesHourly(ctx context.Context) error {
	hourAggregations := make([]string, 24)
	hourColumnNames := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourAggregations[i] = fmt.Sprintf("avgIf(travel_time_seconds, hour = %d) as h%d", i, i)
		hourColumnNames[i] = fmt.Sprintf("h%d", i)
	}

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
	`, strings.Join(hourColumnNames, ",\n\t\t\t"), strings.Join(hourAggregations, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate segment_travel_times_hourly: %w", err)
	}
	log.Println("Populated segment_travel_times_hourly table")
	return nil
}

func (c *Client) populateCumulativeTravelTimes(ctx context.Context) error {
	hourCumulativeAggregations := make([]string, 24)
	cumulativeColumnNames := make([]string, 24)
	for i := 0; i < 24; i++ {
		hourCumulativeAggregations[i] = fmt.Sprintf("sum(h%d) OVER (PARTITION BY hashed_shape_id ORDER BY node_index) as cumulative_h%d", i, i)
		cumulativeColumnNames[i] = fmt.Sprintf("cumulative_h%d", i)
	}

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
	`, strings.Join(cumulativeColumnNames, ",\n\t\t\t"), strings.Join(hourCumulativeAggregations, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate segment_cumulative_times: %w", err)
	}
	log.Println("Populated segment_cumulative_times table")
	return nil
}

func (c *Client) populateRemainingTimes(ctx context.Context) error {
	remainingColumns := make([]string, 24)
	totalColumns := make([]string, 24)
	remainingColumnNames := make([]string, 24)
	cumulativeColumns := make([]string, 24)

	for i := 0; i < 24; i++ {
		remainingColumns[i] = fmt.Sprintf("total_h%d - cumulative_h%d as remaining_h%d", i, i, i)
		totalColumns[i] = fmt.Sprintf("max(cumulative_h%d) OVER (PARTITION BY hashed_shape_id) as total_h%d", i, i)
		remainingColumnNames[i] = fmt.Sprintf("remaining_h%d", i)
		cumulativeColumns[i] = fmt.Sprintf("cumulative_h%d", i)
	}

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
	`, strings.Join(remainingColumnNames, ",\n\t\t\t"),
		strings.Join(remainingColumns, ",\n\t\t\t"),
		strings.Join(cumulativeColumns, ", "),
		strings.Join(totalColumns, ",\n\t\t\t"))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate segment_remaining_times: %w", err)
	}
	log.Println("Populated segment_remaining_times table")
	return nil
}

func (c *Client) populateTimePeriods(ctx context.Context) error {
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
			avgIf(travel_time_seconds, hour IN (7, 8, 9)) as morning_rush,
			sumIf(sample_count, hour IN (7, 8, 9)) as morning_rush_samples,
			avgIf(travel_time_seconds, hour IN (10, 11, 12, 13, 14, 15, 16)) as midday,
			sumIf(sample_count, hour IN (10, 11, 12, 13, 14, 15, 16)) as midday_samples,
			avgIf(travel_time_seconds, hour IN (17, 18, 19)) as evening_rush,
			sumIf(sample_count, hour IN (17, 18, 19)) as evening_rush_samples,
			avgIf(travel_time_seconds, hour IN (20, 21, 22, 23, 0, 1, 2, 3, 4, 5, 6)) as night,
			sumIf(sample_count, hour IN (20, 21, 22, 23, 0, 1, 2, 3, 4, 5, 6)) as night_samples,
			avg(travel_time_seconds) as all_day,
			sum(sample_count) as all_day_samples
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
		ORDER BY hashed_shape_id, node_index
	`

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate segment_time_periods: %w", err)
	}
	log.Println("Populated segment_time_periods table")
	return nil
}

func (c *Client) populateShapeStatistics(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate shape_statistics: %w", err)
	}
	log.Println("Populated shape_statistics table")
	return nil
}

func (c *Client) populateLineStatistics(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to populate line_statistics: %w", err)
	}
	log.Println("Populated line_statistics table")
	return nil
}
