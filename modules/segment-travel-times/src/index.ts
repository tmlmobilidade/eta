/* * */

import type { NodeTravelTimeRecord } from './types.js';

import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';

import { createDefaultSettings, RUN_INTERVAL } from './config.js';
import { createClickHouseClient } from './lib/clickhouse-client.js';
import { createTravelTimesTable, deleteTravelTimesForShapes, saveTravelTimes } from './lib/clickhouse-storage.js';
import { fetchVehicleEvents } from './lib/geohash-cache.js';
import { aggregateRidesToLineShapes } from './lib/line-shapes-aggregator.js';
import { fetchRidesCursor } from './lib/rides-service.js';
import { processShapeTravelTimes } from './lib/travel-time-calculator.js';

/* * */

/**
 * Main processing function for node travel times calculation
 */
async function main(): Promise<void> {
	Logger.init();
	const globalTimer = new Timer();

	// Initialize settings and clients
	const settings = createDefaultSettings();
	const clickhouseClient = createClickHouseClient();

	// Ensure the travel times table exists
	await createTravelTimesTable(clickhouseClient);

	/**
	 * We fetch rides from MongoDB within a data range to know what Shapes were being used in the given period.
	 * This is the node data that's going to be used to calculate the travel times.
	 */
	const { cursor, totalCount } = await fetchRidesCursor(settings);
	Logger.info(`Found ${totalCount} rides`);

	/**
	 * Here we are actually fetching the hashed shapes from the database,
	 * chunking them into nodes of the given length,
	 * and geohashing the coordinates of the endpoints.
	 *
	 * We then group them by their line_id so that we don't fetch the same vehicle events for the same line multiple times.
	 * This could possibly be improved in the future by not fetching the vehicle events for the same geohashe more than once,
	 * but for now we assume that the vehicle events in the same line are going to be mostly in the same geohashes.
	 */
	const { lineShapes } = await aggregateRidesToLineShapes(cursor, settings);
	const sortedLines = Array.from(lineShapes.entries()).sort(([a], [b]) => a - b);

	/**
	 * Here we are processing each line, fetching the vehicle events for the geohashes covered by the line,
	 * We group the events by trip_id, sorted by "created_at" to understand what the event sequence is.
	 *
	 * This will allow us to calculate the travel times for each node, by knowing the sequence of events and the time between them.
	 * - We use bearing to filter events that are traveling in the opposite direction (different route/pattern).
	 * - We discard any events that only have one occurrence (trip_id), as they are not useful for the calculation.
	 * - We calculate the travel time for each node, by knowing the sequence of events and the time between them and attaching it to the nodes.
	 * - i.e. If the node sequence is [A, B, C, D]
	 * - A -> D = 10 seconds
	 * - We assume that each node has travel time of 2.5 seconds, we are splitting the 10 seconds evenly across 4 nodes of 2.5 seconds each.
	 * - Travel times are aggregated by hour (0-23) for time-of-day analysis.
	 */
	for (const [index, [lineId, { geohashes, hashedShapeIds, nodes }]] of sortedLines.entries()) {
		Logger.title(`[${index + 1}/${sortedLines.length}] | Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

		// Fetch the vehicle events for the geohashes covered by the line's shapes
		const allGeohashes = Array.from(geohashes);
		const vehicleEvents = await fetchVehicleEvents(clickhouseClient, allGeohashes, settings);
		Logger.info(`Found ${vehicleEvents.length} vehicle events`);

		if (vehicleEvents.length === 0) {
			Logger.info('No vehicle events found, skipping line');
			Logger.divider();
			continue;
		}

		// Delete existing travel time records for this line's shapes to prevent duplicates
		await deleteTravelTimesForShapes(clickhouseClient, lineId, hashedShapeIds);

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
			await saveTravelTimes(clickhouseClient, allRecords);
			Logger.success(`Saved ${allRecords.length} travel time records for line ${lineId}`);
		}
		else {
			Logger.info(`No travel time records generated for line ${lineId}`);
		}

		Logger.divider();
	}

	Logger.terminate(`Terminated in ${globalTimer.get()}`);
}

/* * */

(async function init(): Promise<void> {
	const runOnInterval = async (): Promise<void> => {
		await main();
		setTimeout(runOnInterval, RUN_INTERVAL);
	};
	runOnInterval();
})();

/* * */
