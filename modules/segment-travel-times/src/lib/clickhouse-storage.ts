/* * */

import type { NodeTravelTimeRecord } from '../types.js';

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';

/* * */

/**
 * Creates the segment_travel_times table in ClickHouse if it doesn't exist
 *
 * @param client - ClickHouse client
 */
export async function createTravelTimesTable(client: ClickHouseClient): Promise<void> {
	const query = `
		CREATE TABLE IF NOT EXISTS segment_travel_times (
			line_id UInt32,
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			hour UInt8,
			travel_time_seconds Float32,
			sample_count UInt32
		) ENGINE = MergeTree()
		ORDER BY (line_id, hashed_shape_id, node_index, hour)
	`;

	await client.command({ query });
	Logger.info('Created/verified segment_travel_times table');
}

/**
 * Saves travel time records to ClickHouse
 * Uses batch insert for efficiency
 *
 * @param client - ClickHouse client
 * @param records - Array of travel time records to insert
 */
export async function saveTravelTimes(client: ClickHouseClient, records: NodeTravelTimeRecord[]): Promise<void> {
	if (records.length === 0) {
		return;
	}

	await client.insert({
		format: 'JSONEachRow',
		table: 'segment_travel_times',
		values: records,
	});

	Logger.info(`Saved ${records.length} travel time records to ClickHouse`);
}

/**
 * Deletes existing travel time records for specific line_id and hashed_shape_ids
 * Used to prevent duplicate data when reprocessing
 *
 * @param client - ClickHouse client
 * @param lineId - Line ID to delete records for
 * @param hashedShapeIds - Array of hashed shape IDs to delete records for
 */
export async function deleteTravelTimesForShapes(
	client: ClickHouseClient,
	lineId: number,
	hashedShapeIds: string[],
): Promise<void> {
	if (hashedShapeIds.length === 0) {
		return;
	}

	const query = `
		ALTER TABLE segment_travel_times 
		DELETE WHERE line_id = ${lineId} 
		AND hashed_shape_id IN ('${hashedShapeIds.join('\',\'')}')
	`;

	await client.command({ query });
	Logger.info(`Deleted existing travel time records for line ${lineId} with ${hashedShapeIds.length} shapes`);
}

/* * */
