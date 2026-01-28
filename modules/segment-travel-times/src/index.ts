/* * */

import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';

import { createDefaultSettings, RUN_INTERVAL } from './config.js';
import { createAggregationTables } from './lib/clickhouse-aggregations.js';
import { createClickHouseClient } from './lib/clickhouse-client.js';
import { createTravelTimesTable } from './lib/clickhouse-storage.js';
import { processAllLines } from './lib/line-processor.js';
import { aggregateRidesToLineShapes } from './lib/line-shapes-aggregator.js';
import { fetchRidesCursor } from './lib/rides-service.js';

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
	 * Process each line to calculate and save travel times.
	 * See processLine() function for detailed documentation on the processing logic.
	 */
	await processAllLines(clickhouseClient, sortedLines, settings);

	// Create aggregation tables (pivot hourly data)
	await createAggregationTables(clickhouseClient);

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
