/* * */

import type { LineShapeData, NodeTravelTimeRecord, SegmentTravelTimesSettings, VehicleEvent } from '../types.js';

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';

import { deleteTravelTimesForShapes, saveTravelTimes } from './clickhouse-storage.js';
import { fetchVehicleEvents } from './geohash-cache.js';
import { processShapeTravelTimes } from './travel-time-calculator.js';

/* * */

/**
 * Calculates and aggregates travel times across all shapes for a given line.
 *
 * Workflow:
 * - Uses provided vehicle events relevant to the line's geohashes.
 * - Organizes events by trip_id, sorting by their timestamp (created_at) to reconstruct traversal sequences.
 * - Computes segment travel times per node, filtering out opposite direction trips using a bearing threshold.
 * - Ignores trip_ids that appear only once, as they're insufficient for travel time calculation.
 * - Evenly distributes total travel time across all traversed nodes (e.g., total time for A→D is split among nodes A-B, B-C, C-D).
 * - Aggregates computed travel times by hour of day (0–23) for temporal analysis.
 *
 * @param client - ClickHouse client
 * @param lineId - The line ID being processed
 * @param lineData - The line's shape data (geohashes, hashed shape IDs, nodes)
 * @param settings - Processing settings including bearing threshold
 * @param index - Current index in the processing loop (for logging)
 * @param totalLines - Total number of lines being processed (for logging)
 */
export async function processLine(vehicleEvents: VehicleEvent[], lineId: number, lineData: LineShapeData, settings: SegmentTravelTimesSettings): Promise<NodeTravelTimeRecord[]> {
	const { hashedShapeIds, nodes } = lineData;

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

	return allRecords;
}

/**
 * Fetches vehicle events and prepares the line for processing
 *
 * @param client - ClickHouse client
 * @param geohashes - Set of geohashes covered by the line's shapes
 * @param lineId - The line ID being processed
 * @param hashedShapeIds - Array of hashed shape IDs for this line
 * @param settings - Processing settings
 * @returns Vehicle events array, or null if no events found
 */
async function fetchAndPrepareVehicleEvents(
	client: ClickHouseClient,
	geohashes: Set<string>,
	lineId: number,
	hashedShapeIds: string[],
	settings: SegmentTravelTimesSettings,
): Promise<null | VehicleEvent[]> {
	// Fetch the vehicle events for the geohashes covered by the line's shapes
	const allGeohashes = Array.from(geohashes);
	const vehicleEvents = await fetchVehicleEvents(client, allGeohashes, settings);

	if (vehicleEvents.length === 0) return null;

	// Delete existing travel time records for this line's shapes to prevent duplicates
	await deleteTravelTimesForShapes(client, lineId, hashedShapeIds);

	return vehicleEvents;
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
		const { geohashes, hashedShapeIds } = lineData;
		const timer = new Timer();

		Logger.title(`[${index + 1}/${sortedLines.length}] | Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

		const vehicleEvents = await fetchAndPrepareVehicleEvents(client, geohashes, lineId, hashedShapeIds, settings);
		if (!vehicleEvents) {
			Logger.info('No vehicle events found, skipping line');
			Logger.divider();
			return;
		}

		Logger.info(`Found ${vehicleEvents.length} vehicle events | [${timer.get()}]`);

		timer.reset();
		const allRecords = await processLine(vehicleEvents, lineId, lineData, settings);
		Logger.info(`Processed ${allRecords.length} travel time records | [${timer.get()}]`);

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
}

/* * */
