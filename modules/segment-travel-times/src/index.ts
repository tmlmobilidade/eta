/* * */

import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';

import { createDefaultSettings, RUN_INTERVAL } from './config.js';
import { createClickHouseClient } from './lib/clickhouse-client.js';
import { fetchVehicleEvents } from './lib/geohash-cache.js';
import { aggregateRidesToLineShapes } from './lib/line-shapes-aggregator.js';
import { fetchRidesCursor } from './lib/rides-service.js';

/* * */

/**
 * Main processing function for segment travel times calculation
 *
 * Workflow:
 * 1. Fetch rides from MongoDB within date range
 * 2. Group shapes by line_id with segment endpoints and geohashes
 * 3. For each line, fetch uncached vehicle events from ClickHouse
 * 4. Aggregate events by trip_id for analysis
 */
async function main(): Promise<void> {
	Logger.init();
	const globalTimer = new Timer();

	// Initialize settings and clients
	const settings = createDefaultSettings();
	const clickhouseClient = createClickHouseClient();

	// Step 1: Fetch rides cursor
	const { cursor, totalCount } = await fetchRidesCursor(settings);

	Logger.info(`Found ${totalCount} rides`);

	// Step 2: Aggregate rides into line shapes
	const { lineShapes } = await aggregateRidesToLineShapes(cursor, settings);

	// Sort lines by ID for consistent processing order
	const sortedLines = Array.from(lineShapes.entries()).sort(([a], [b]) => a - b);

	for (const [index, [lineId, { geohashes, hashedShapeIds }]] of sortedLines.entries()) {
		Logger.title(`[${index + 1}/${sortedLines.length}] | Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

		const allGeohashes = Array.from(geohashes);
		const vehicleEvents = await fetchVehicleEvents(clickhouseClient, allGeohashes, settings);

		Logger.info(`Found ${vehicleEvents.length} vehicle events`);

		// // Step 3a: Fetch and cache uncached geohashes
		// const groupedEvents = aggregateEventsByTripId(allGeohashes, vehicleEvents);

		// console.log(groupedEvents);
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
