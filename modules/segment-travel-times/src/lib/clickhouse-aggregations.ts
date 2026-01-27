/* * */

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';

/* * */

/**
 * Creates the hourly pivot table for travel times
 * Columns: hashed_shape_id, node_index, latitude, longitude, h0-h23 (travel times)
 *
 * @param client - ClickHouse client
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
	Logger.info('Created/verified segment_travel_times_hourly table');
}

/**
 * Creates the hourly pivot table for sample counts
 * Columns: hashed_shape_id, node_index, h0-h23 (sample counts)
 *
 * @param client - ClickHouse client
 */
export async function createSampleCountsHourlyTable(client: ClickHouseClient): Promise<void> {
	const hourColumns = Array.from({ length: 24 }, (_, i) => `h${i} UInt32`).join(',\n\t\t\t');

	const query = `
		CREATE TABLE IF NOT EXISTS segment_sample_counts_hourly (
			hashed_shape_id String,
			node_index UInt16,
			${hourColumns}
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`;

	await client.command({ query });
	Logger.info('Created/verified segment_sample_counts_hourly table');
}

/**
 * Creates a combined pivot table with both travel times and sample counts per hour
 * Columns: hashed_shape_id, node_index, latitude, longitude,
 *          tt_h0-tt_h23 (travel times), sc_h0-sc_h23 (sample counts)
 *
 * @param client - ClickHouse client
 */
export async function createCombinedHourlyTable(client: ClickHouseClient): Promise<void> {
	const travelTimeColumns = Array.from({ length: 24 }, (_, i) => `tt_h${i} Float32`).join(',\n\t\t\t');
	const sampleCountColumns = Array.from({ length: 24 }, (_, i) => `sc_h${i} UInt32`).join(',\n\t\t\t');

	const query = `
		CREATE TABLE IF NOT EXISTS segment_hourly_combined (
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			${travelTimeColumns},
			${sampleCountColumns}
		) ENGINE = ReplacingMergeTree()
		ORDER BY (hashed_shape_id, node_index)
	`;

	await client.command({ query });
	Logger.info('Created/verified segment_hourly_combined table');
}

/**
 * Populates the travel times hourly pivot table from segment_travel_times
 *
 * @param client - ClickHouse client
 */
export async function populateTravelTimesHourly(client: ClickHouseClient): Promise<void> {
	// Generate the hour aggregation columns
	const hourAggregations = Array.from(
		{ length: 24 },
		(_, i) => `avgIf(travel_time_seconds, hour = ${i}) as h${i}`,
	).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_travel_times_hourly (
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			${Array.from({ length: 24 }, (_, i) => `h${i}`).join(',\n\t\t\t')}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			any(latitude) as latitude,
			any(longitude) as longitude,
			${hourAggregations}
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_travel_times_hourly table');
}

/**
 * Populates the sample counts hourly pivot table from segment_travel_times
 *
 * @param client - ClickHouse client
 */
export async function populateSampleCountsHourly(client: ClickHouseClient): Promise<void> {
	// Generate the hour aggregation columns
	const hourAggregations = Array.from(
		{ length: 24 },
		(_, i) => `sumIf(sample_count, hour = ${i}) as h${i}`,
	).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_sample_counts_hourly (
			hashed_shape_id,
			node_index,
			${Array.from({ length: 24 }, (_, i) => `h${i}`).join(',\n\t\t\t')}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			${hourAggregations}
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_sample_counts_hourly table');
}

/**
 * Populates the combined hourly pivot table from segment_travel_times
 *
 * @param client - ClickHouse client
 */
export async function populateCombinedHourly(client: ClickHouseClient): Promise<void> {
	// Generate the travel time aggregation columns
	const travelTimeAggregations = Array.from(
		{ length: 24 },
		(_, i) => `avgIf(travel_time_seconds, hour = ${i}) as tt_h${i}`,
	).join(',\n\t\t\t');

	// Generate the sample count aggregation columns
	const sampleCountAggregations = Array.from(
		{ length: 24 },
		(_, i) => `sumIf(sample_count, hour = ${i}) as sc_h${i}`,
	).join(',\n\t\t\t');

	const travelTimeColumnNames = Array.from({ length: 24 }, (_, i) => `tt_h${i}`).join(',\n\t\t\t');
	const sampleCountColumnNames = Array.from({ length: 24 }, (_, i) => `sc_h${i}`).join(',\n\t\t\t');

	const query = `
		INSERT INTO segment_hourly_combined (
			hashed_shape_id,
			node_index,
			latitude,
			longitude,
			${travelTimeColumnNames},
			${sampleCountColumnNames}
		)
		SELECT 
			hashed_shape_id,
			node_index,
			any(latitude) as latitude,
			any(longitude) as longitude,
			${travelTimeAggregations},
			${sampleCountAggregations}
		FROM segment_travel_times
		GROUP BY hashed_shape_id, node_index
	`;

	await client.command({ query });
	Logger.info('Populated segment_hourly_combined table');
}

/**
 * Drops the aggregation tables if they exist
 * Used to recreate tables with correct schema
 *
 * @param client - ClickHouse client
 */
export async function dropAggregationTables(client: ClickHouseClient): Promise<void> {
	await client.command({ query: 'DROP TABLE IF EXISTS segment_travel_times_hourly' });
	await client.command({ query: 'DROP TABLE IF EXISTS segment_sample_counts_hourly' });
	await client.command({ query: 'DROP TABLE IF EXISTS segment_hourly_combined' });
	Logger.info('Dropped existing aggregation tables');
}

/**
 * Truncates the aggregation tables before repopulating
 *
 * @param client - ClickHouse client
 */
export async function truncateAggregationTables(client: ClickHouseClient): Promise<void> {
	await client.command({ query: 'TRUNCATE TABLE IF EXISTS segment_travel_times_hourly' });
	await client.command({ query: 'TRUNCATE TABLE IF EXISTS segment_sample_counts_hourly' });
	await client.command({ query: 'TRUNCATE TABLE IF EXISTS segment_hourly_combined' });
	Logger.info('Truncated aggregation tables');
}

/**
 * Creates all aggregation tables and populates them with data
 * This should be called after all travel times have been calculated
 *
 * @param client - ClickHouse client
 */
export async function createAggregationTables(client: ClickHouseClient): Promise<void> {
	Logger.title('Creating aggregation tables');

	// Drop existing tables to ensure correct schema
	await dropAggregationTables(client);

	// Create all tables
	await createTravelTimesHourlyTable(client);
	await createSampleCountsHourlyTable(client);
	await createCombinedHourlyTable(client);

	// Populate with fresh data
	await populateTravelTimesHourly(client);
	await populateSampleCountsHourly(client);
	await populateCombinedHourly(client);

	Logger.success('Aggregation tables created and populated');
}

/* * */
