/* * */

import type { LineShapeData, NodeTravelTimeRecord, SegmentTravelTimesSettings } from '../types.js';

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';

import { deleteTravelTimesForShapes, saveTravelTimes } from './clickhouse-storage.js';
import { fetchVehicleEvents } from './geohash-cache.js';
import { processShapeTravelTimes } from './travel-time-calculator.js';

/* * */

/**
 * Processes a single line to calculate and save travel times
 *
 * This function:
 * - Fetches vehicle events for the geohashes covered by the line's shapes
 * - Groups events by trip_id, sorted by "created_at" to understand event sequence
 * - Calculates travel times for each node using bearing to filter opposite-direction events
 * - Discards events with only one occurrence (trip_id) as they're not useful
 * - Splits travel time evenly across nodes (e.g., if A->D = 10s, each node gets 2.5s)
 * - Aggregates travel times by hour (0-23) for time-of-day analysis
 *
 * @param client - ClickHouse client
 * @param lineId - The line ID being processed
 * @param lineData - The line's shape data (geohashes, hashed shape IDs, nodes)
 * @param settings - Processing settings including bearing threshold
 * @param index - Current index in the processing loop (for logging)
 * @param totalLines - Total number of lines being processed (for logging)
 */
export async function processLine(
	client: ClickHouseClient,
	lineId: number,
	lineData: LineShapeData,
	settings: SegmentTravelTimesSettings,
	index: number,
	totalLines: number,
): Promise<void> {
	const { geohashes, hashedShapeIds, nodes } = lineData;

	Logger.title(`[${index + 1}/${totalLines}] | Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

	// Fetch the vehicle events for the geohashes covered by the line's shapes
	const allGeohashes = Array.from(geohashes);
	const vehicleEvents = await fetchVehicleEvents(client, allGeohashes, settings);
	Logger.info(`Found ${vehicleEvents.length} vehicle events`);

	if (vehicleEvents.length === 0) {
		Logger.info('No vehicle events found, skipping line');
		Logger.divider();
		return;
	}

	// Delete existing travel time records for this line's shapes to prevent duplicates
	await deleteTravelTimesForShapes(client, lineId, hashedShapeIds);

	// Process each hashed shape and calculate travel times
	const allRecords: NodeTravelTimeRecord[] = [];
	let processedShapes = 0;
	let totalSamples = 0;

	for (const hashedShapeId of hashedShapeIds) {
		const shapeNodes = nodes.get(hashedShapeId);
		if (!shapeNodes || shapeNodes.length < 2) {
			continue;
		}

		// Calculate travel times for this shape
		const records = processShapeTravelTimes(
			vehicleEvents,
			shapeNodes,
			lineId,
			hashedShapeId,
			settings.bearingThreshold,
		);

		if (records.length > 0) {
			allRecords.push(...records);
			processedShapes++;
			totalSamples += records.reduce((sum, r) => sum + r.sample_count, 0);
		}
	}

	Logger.info(`Processed ${processedShapes}/${hashedShapeIds.length} shapes with ${totalSamples} total samples`);

	// Save all records for this line to ClickHouse
	if (allRecords.length > 0) {
		await saveTravelTimes(client, allRecords);
		Logger.success(`Saved ${allRecords.length} travel time records for line ${lineId}`);
	}
	else {
		Logger.info(`No travel time records generated for line ${lineId}`);
	}

	Logger.divider();
}

/**
 * Processes all lines sequentially
 *
 * @param client - ClickHouse client
 * @param sortedLines - Array of [lineId, lineData] tuples, sorted by lineId
 * @param settings - Processing settings
 */
export async function processAllLines(client: ClickHouseClient, sortedLines: [number, LineShapeData][], settings: SegmentTravelTimesSettings): Promise<void> {
	for (const [index, [lineId, lineData]] of sortedLines.entries()) {
		await processLine(client, lineId, lineData, settings, index, sortedLines.length);
	}
}

/* * */
