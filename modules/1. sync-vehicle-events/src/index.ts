/* * */

import { createClient } from '@clickhouse/client';
import { Dates } from '@tmlmobilidade/dates';
import { rides, simplifiedVehicleEvents } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { ClickHouseWriter } from '@tmlmobilidade/writers';

import { parseToEtaVehicleEvent } from './parser.js';
import { EtaVehicleEvent, EtaVehicleEventTableSchema } from './types.js';

/* * */

const RUN_INTERVAL = 60_000 * 15; // 15 minutes in milliseconds

async function main() {
	//

	Logger.init();

	const globalTimer = new Timer();

	//
	// Setup Clickhouse

	const clickhouseClient = createClient({
		database: process.env.CLICKHOUSE_DATABASE,
		password: process.env.CLICKHOUSE_PASSWORD,
		url: `${process.env.CLICKHOUSE_TLS === 'true' ? 'https' : 'http'}://${process.env.CLICKHOUSE_HOST}:8123`,
		username: process.env.CLICKHOUSE_USERNAME,
	});

	// Drop the vehicle_events table if it exists for a clean start
	await clickhouseClient.command({ query: `DROP TABLE IF EXISTS vehicle_events` });

	const clickhouseWriter = new ClickHouseWriter<EtaVehicleEvent>({
		batch_size: 100_000,
		client: clickhouseClient,
		table: 'vehicle_events',
		tableSchema: EtaVehicleEventTableSchema,
	});

	//
	// Set Date range for the data we need to fetch

	const now = Dates.now('Europe/Lisbon');
	const START_DATE = now.minus({ days: 7 }).set({ hour: 4, minute: 0, second: 0 });
	// If now is before 04:00, use yesterday's 4:00 AM
	const END_DATE = now.set({ hour: 4, minute: 0, second: 0 }).unix_timestamp > now.unix_timestamp
		? now.minus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 })
		: now.set({ hour: 4, minute: 0, second: 0 });

	//
	// Fetch Rides and add them to a map

	//
	// Get the rides collection

	const ridesCollection = await rides.getCollection();
	const ridesQuery = {
		agency_id: { $in: ['41', '42', '43', '44'] },
		start_time_observed: { $ne: null },
		start_time_scheduled: { $gte: START_DATE.unix_timestamp, $lt: END_DATE.unix_timestamp },
	};

	const ridesCount = await ridesCollection.countDocuments(ridesQuery);
	const ridesCursor = ridesCollection.find(ridesQuery, { projection: { _id: 1, end_time_observed: 1, hashed_shape_id: 1, operational_date: 1, start_time_observed: 1, trip_id: 1 } }).batchSize(30_000);
	const simplifiedVehicleEventsCursor = await simplifiedVehicleEvents.getCollection();

	Logger.info(`Found ${ridesCount} rides, processing...`);

	let processedCount = 0;
	for await (const ride of ridesCursor) {
		const vehicleEventsQuery = {
			created_at: { $gte: ride.start_time_observed, $lte: ride.end_time_observed },
			trip_id: ride.trip_id,
		};
		const vehicleEvents = simplifiedVehicleEventsCursor.find(vehicleEventsQuery).batchSize(100_000);
		const vehicleEventsCount = await simplifiedVehicleEventsCursor.countDocuments(vehicleEventsQuery);

		Logger.info(`Found ${vehicleEventsCount} vehicle events for ride ${ride._id}, processing...`);

		let processedVehicleEventsCount = 0;
		for await (const vehicleEvent of vehicleEvents) {
			const etaVehicleEvent = parseToEtaVehicleEvent(vehicleEvent, ride.hashed_shape_id);
			await clickhouseWriter.write(etaVehicleEvent);
			processedVehicleEventsCount++;
			if (processedVehicleEventsCount % 50_000 === 0) {
				Logger.progress(`Processed ${processedVehicleEventsCount} of ${vehicleEventsCount} vehicle events...`);
			}
		}

		processedCount++;
		if (processedCount % 10_000 === 0) {
			Logger.progress(`Processed ${processedCount} of ${ridesCount} rides...`);
		}
	}

	// Flush the clickhouse writer
	await clickhouseWriter.flush();

	Logger.success(`Processed ${processedCount} of ${ridesCount} rides`);
	Logger.terminate(`Terminated in ${globalTimer.get()}`);
}

/* * */

(async function init() {
	const runOnInterval = async () => {
		await main();
		setTimeout(runOnInterval, RUN_INTERVAL);
	};
	runOnInterval();
})();
