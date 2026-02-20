/* * */

import { createClient } from '@clickhouse/client';
import { Dates } from '@tmlmobilidade/dates';
import { Filter, rides, simplifiedVehicleEvents } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { Ride } from '@tmlmobilidade/types';
import { ClickHouseWriter } from '@tmlmobilidade/writers';

import { parseToEtaVehicleEvent } from './parser.js';
import { EtaVehicleEvent, EtaVehicleEventTableSchema, rideProjection } from './types.js';

/* * */

const RUN_INTERVAL_MS = 60_000 * 60 * 24; // 24 hours
const RIDES_BATCH_SIZE = 30_000;
const AGENCY_IDS = ['41', '42', '43', '44'];

const BATCH_SIZE = 50_000;

/* * */

function getDateRange(): { end: Dates, start: Dates } {
	//

	const now = Dates.now('Europe/Lisbon');
	const start = now.minus({ days: 7 }).set({ hour: 4, minute: 0, second: 0 });
	const endCutoff = now.set({ hour: 4, minute: 0, second: 0 });
	const end = endCutoff.unix_timestamp > now.unix_timestamp
		? now.minus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 })
		: endCutoff;

	//

	return { end, start };
}

function createClickHouseWriter(client: ReturnType<typeof createClient>) {
	//

	return new ClickHouseWriter<EtaVehicleEvent>({
		batch_size: BATCH_SIZE,
		client,
		table: 'vehicle_events',
		tableSchema: EtaVehicleEventTableSchema,
	});
}

async function syncVehicleEvents(writer: ClickHouseWriter<EtaVehicleEvent>, start: Dates, end: Dates): Promise<{ eventsProcessed: number, ridesProcessed: number }> {
	//

	const ridesCollection = await rides.getCollection();
	const vehicleEventsCollection = await simplifiedVehicleEvents.getCollection();

	//
	// Setup Rides Cursor

	const ridesQuery: Filter<Ride> = {
		agency_id: { $in: AGENCY_IDS },
		// line_id: { $in: [1001, 1002] }, // ! Development only
		start_time_observed: { $ne: null },
		start_time_scheduled: { $gte: start.unix_timestamp, $lt: end.unix_timestamp },
	};

	const ridesCount = await ridesCollection.countDocuments(ridesQuery);

	Logger.info(`Syncing vehicle events for ${ridesCount} rides (${start.toFormat('yyyy-MM-dd')} → ${end.toFormat('yyyy-MM-dd')})`);

	let ridesProcessed = 0;
	let eventsProcessed = 0;

	// Process rides in batches to avoid cursor timeout
	while (ridesProcessed < ridesCount) {
		const ridesBatch = await ridesCollection
			.find(ridesQuery, { projection: rideProjection })
			.skip(ridesProcessed)
			.limit(RIDES_BATCH_SIZE)
			.toArray();

		if (ridesBatch.length === 0) break;

		// Process each ride in the batch
		for (const ride of ridesBatch) {
			const vehicleEventsCursor = vehicleEventsCollection
				.find({
					created_at: { $gte: ride.start_time_observed, $lte: ride.end_time_observed },
					trip_id: ride.trip_id,
				})
				.batchSize(BATCH_SIZE);

			for await (const vehicleEvent of vehicleEventsCursor) {
				await writer.write(parseToEtaVehicleEvent(vehicleEvent, ride));
				eventsProcessed++;
				if (eventsProcessed % BATCH_SIZE === 0) {
					Logger.progress(`Processed a total of ${eventsProcessed} events from ${ridesProcessed + ridesBatch.indexOf(ride) + 1} rides`);
				}
			}
		}

		ridesProcessed += ridesBatch.length;
		Logger.progress(`Completed batch: ${ridesProcessed}/${ridesCount} rides processed`);
	}

	return { eventsProcessed, ridesProcessed };
}

async function main(): Promise<void> {
	//

	Logger.init();
	const timer = new Timer();

	//
	// Setup Clickhouse
	const client = createClient({
		database: process.env.CLICKHOUSE_DATABASE,
		password: process.env.CLICKHOUSE_PASSWORD,
		url: `${process.env.CLICKHOUSE_TLS === 'true' ? 'https' : 'http'}://${process.env.CLICKHOUSE_HOST}:8123`,
		username: process.env.CLICKHOUSE_USERNAME,
	});

	await client.command({ query: 'DROP TABLE IF EXISTS vehicle_events' });

	const writer = createClickHouseWriter(client);
	await writer.ensureTable(); // Creates table if it doesn't exist

	//
	// Get Date Range
	const { end, start } = getDateRange();

	//
	// Sync Vehicle Events
	const { eventsProcessed, ridesProcessed } = await syncVehicleEvents(writer, start, end);

	//
	// Flush Writer Buffer
	await writer.flush();

	Logger.success(`Sync completed: ${ridesProcessed} rides, ${eventsProcessed} events in ${timer.get()}`);
	Logger.terminate(`Terminated in ${timer.get()}`);
}

/* * */

(async function run() {
	const runOnInterval = async () => {
		await main();
		setTimeout(runOnInterval, RUN_INTERVAL_MS);
	};
	runOnInterval();
})();
