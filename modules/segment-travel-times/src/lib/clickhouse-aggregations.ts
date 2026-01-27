/* * */

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';

/* * */

// ============================================================================
// TABLE CREATION FUNCTIONS
// ============================================================================

/**
 * Creates the hourly pivot table for travel times
 * Columns: hashed_shape_id, node_index, latitude, longitude, h0-h23 (travel times)
 * USE CASE: Get travel time for a specific segment at a specific hour
 */
export async function createTravelTimesHourlyTable(client: ClickHouseClient): Promise<void> {
	const hourColumns = Array.from({ length: 24 }, (_, i) => `h${i} Float32`).join(',\n\t\t\t');

	const query = `
		CREATE TABLE IF NOT EXISTS segment_travel_times_hourly (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			${hourColumns}
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`;

	await client.command({ query });
	Logger.info('Created segment_travel_times_hourly table');
}

/**
 * Creates cumulative travel times table (time from start to each node)
 * USE CASE: Calculate ETA by subtracting current position from total route time
 * Example: If total route = 1800s and cumulative at node 5 = 300s, remaining = 1500s
 */
export async function createCumulativeTravelTimesTable(client: ClickHouseClient): Promise<void> {
	const hourColumns = Array.from({ length: 24 }, (_, i) => `cumulative_h${i} Float32`).join(',\n\t\t\t');

	const query = `
		CREATE TABLE IF NOT EXISTS segment_cumulative_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			${hourColumns}
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`;

	await client.command({ query });
	Logger.info('Created segment_cumulative_times table');
}

/**
 * Creates remaining time to end table (time from each node to route end)
 * USE CASE: Direct ETA lookup - given current node and hour, get remaining time
 * This is the most useful table for real-time ETA predictions
 */
export async function createRemainingTimeTable(client: ClickHouseClient): Promise<void> {
	const hourColumns = Array.from({ length: 24 }, (_, i) => `remaining_h${i} Float32`).join(',\n\t\t\t');

	const query = `
		CREATE TABLE IF NOT EXISTS segment_remaining_times (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			${hourColumns}
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`;

	await client.command({ query });
	Logger.info('Created segment_remaining_times table');
}

/**
 * Creates time period aggregation table (rush hour, midday, night)
 * USE CASE: Simpler queries when exact hour granularity isn't needed
 * Time periods:
 *   - morning_rush: 7-9 (peak congestion)
 *   - midday: 10-16 (moderate traffic)
 *   - evening_rush: 17-19 (peak congestion)
 *   - night: 20-6 (low traffic)
 */
export async function createTimePeriodTable(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Created segment_time_periods table');
}

/**
 * Creates shape statistics table (route-level aggregations)
 * USE CASE: Quick route overview, fallback estimates, data quality checks
 * Contains: total nodes, total travel time per period, average speed estimates
 */
export async function createShapeStatisticsTable(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Created shape_statistics table');
}

/**
 * Creates line-level statistics table (fallback when shape data is unavailable)
 * USE CASE: Fallback estimates when specific shape has no data
 */
export async function createLineStatisticsTable(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Created line_statistics table');
}

// ============================================================================
// POPULATION FUNCTIONS
// ============================================================================

/**
 * Populates the hourly travel times pivot table
 */
export async function populateTravelTimesHourly(client: ClickHouseClient): Promise<void> {
	const hourAggregations = Array.from(
		{ length: 24 },
		(_, i) => `avgIf(travel_time_seconds, hour = ${i}) as h${i}`,
	).join(',\n\t\t\t');

	const hourColumnNames = Array.from({ length: 24 }, (_, i) => `h${i}`).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_travel_times_hourly (
			hashed_shape_id, node_index, latitude, longitude,
			${hourColumnNames}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			any(latitude) as latitude,
			any(longitude) as longitude,
			${hourAggregations}
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
		ORDER BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_travel_times_hourly table');
}

/**
 * Populates cumulative travel times (running sum from start to each node)
 */
export async function populateCumulativeTravelTimes(client: ClickHouseClient): Promise<void> {
	const hourCumulativeAggregations = Array.from(
		{ length: 24 },
		(_, i) => `sum(h${i}) OVER (PARTITION BY hashed_shape_id ORDER BY node_index) as cumulative_h${i}`,
	).join(',\n\t\t\t');

	const cumulativeColumnNames = Array.from({ length: 24 }, (_, i) => `cumulative_h${i}`).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_cumulative_times (
			hashed_shape_id, node_index, latitude, longitude,
			${cumulativeColumnNames}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			${hourCumulativeAggregations}
		FROM segment_travel_times_hourly
		ORDER BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_cumulative_times table');
}

/**
 * Populates remaining time to end (total route time - cumulative time at each node)
 */
export async function populateRemainingTimes(client: ClickHouseClient): Promise<void> {
	const remainingColumns = Array.from(
		{ length: 24 },
		(_, i) => `total_h${i} - cumulative_h${i} as remaining_h${i}`,
	).join(',\n\t\t\t');

	const totalColumns = Array.from(
		{ length: 24 },
		(_, i) => `max(cumulative_h${i}) OVER (PARTITION BY hashed_shape_id) as total_h${i}`,
	).join(',\n\t\t\t');

	const remainingColumnNames = Array.from({ length: 24 }, (_, i) => `remaining_h${i}`).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_remaining_times (
			hashed_shape_id, node_index, latitude, longitude,
			${remainingColumnNames}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			${remainingColumns}
		FROM (
			SELECT 
				hashed_shape_id,
				node_index,
				latitude,
				longitude,
				${Array.from({ length: 24 }, (_, i) => `cumulative_h${i}`).join(', ')},
				${totalColumns}
			FROM segment_cumulative_times
		)
		ORDER BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_remaining_times table');
}

/**
 * Populates time period aggregations
 * Morning rush: hours 7, 8, 9
 * Midday: hours 10-16
 * Evening rush: hours 17, 18, 19
 * Night: hours 20-23, 0-6
 */
export async function populateTimePeriods(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Populated segment_time_periods table');
}

/**
 * Populates shape-level statistics
 */
export async function populateShapeStatistics(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Populated shape_statistics table');
}

/**
 * Populates line-level statistics (averages across all shapes in a line)
 */
export async function populateLineStatistics(client: ClickHouseClient): Promise<void> {
	const query = `
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
	`;

	await client.command({ query });
	Logger.info('Populated line_statistics table');
}

// ============================================================================
// MANAGEMENT FUNCTIONS
// ============================================================================

/**
 * Drops all aggregation tables
 */
export async function dropAggregationTables(client: ClickHouseClient): Promise<void> {
	const tables = [
		'segment_travel_times_hourly',
		'segment_cumulative_times',
		'segment_remaining_times',
		'segment_time_periods',
		'shape_statistics',
		'line_statistics',
	];

	for (const table of tables) {
		await client.command({ query: `DROP TABLE IF EXISTS ${table}` });
	}

	Logger.info('Dropped existing aggregation tables');
}

/**
 * Creates and populates all aggregation tables
 * Tables are created in dependency order (some tables depend on others)
 *
 * @param client - ClickHouse client
 */
export async function createAggregationTables(client: ClickHouseClient): Promise<void> {
	Logger.title('Creating ETA aggregation tables');

	// Drop existing tables to ensure correct schema
	await dropAggregationTables(client);

	// Create all tables
	await createTravelTimesHourlyTable(client);
	await createCumulativeTravelTimesTable(client);
	await createRemainingTimeTable(client);
	await createTimePeriodTable(client);
	await createShapeStatisticsTable(client);
	await createLineStatisticsTable(client);

	// Populate in dependency order
	// 1. Base hourly pivot (no dependencies)
	await populateTravelTimesHourly(client);

	// 2. Cumulative times (depends on hourly)
	await populateCumulativeTravelTimes(client);

	// 3. Remaining times (depends on cumulative)
	await populateRemainingTimes(client);

	// 4. Time periods (no dependencies, from base table)
	await populateTimePeriods(client);

	// 5. Shape statistics (depends on time periods)
	await populateShapeStatistics(client);

	// 6. Line statistics (depends on shape statistics)
	await populateLineStatistics(client);

	Logger.success('ETA aggregation tables created and populated');
}

/* * */
