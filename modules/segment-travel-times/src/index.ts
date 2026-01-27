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
 */
async function main(): Promise<void> {
	Logger.init();
	const globalTimer = new Timer();

	// Initialize settings and clients
	const settings = createDefaultSettings();
	const clickhouseClient = createClickHouseClient();

	/**
	 * We fetch rides from MongoDB within a data range to know what Shapes were being used in the given period.
	 * This is the segment data that's going to be used to calculate the travel times.
	 */
	const { cursor, totalCount } = await fetchRidesCursor(settings);
	Logger.info(`Found ${totalCount} rides`);

	/**
	 * Here we are actually fetching the hashed shapes from the database,
	 * chunking them into segments of the given length,
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
	 * We groupe the events by trip_id, sorted by "created_at" to understand what the event sequence is.
	 *
	 * This will allow us to calculate the travel times for each segment, by knowing the sequence of events and the time between them.
	 * - We discard any events that only have one occurrence (trip_id), as they are not useful for the calculation.
	 * - We calculate the travel time for each segment, by knowing the sequence of events and the time between them and attaching it to the segments.
	 * - i.e. If the event sequence is [A, B, C, D]
	 * - A -> D = 10 seconds
	 * - We assume that each segment is has travel time of 2,5 seconds, we are splitting the 10 seconds evenly across 4 segments of 2,5 seconds each.
	 */
	for (const [index, [lineId, { geohashes, hashedShapeIds }]] of sortedLines.entries()) {
		Logger.title(`[${index + 1}/${sortedLines.length}] | Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

		const allGeohashes = Array.from(geohashes);
		const vehicleEvents = await fetchVehicleEvents(clickhouseClient, allGeohashes, settings);

		Logger.info(`Found ${vehicleEvents.length} vehicle events`);

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
