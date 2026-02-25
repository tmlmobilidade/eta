/* * */

import { createClient } from '@clickhouse/client';
import { Dates } from '@tmlmobilidade/dates';
import { Filter, hashedShapes, rides, simplifiedVehicleEvents } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { Ride } from '@tmlmobilidade/types';
import { ClickHouseWriter } from '@tmlmobilidade/writers';
import * as turf from '@turf/turf';

import { hashedShapesToFeatureCollection } from './hashed-shapes-to-geojson.js';
import { parseToEtaVehicleEvent } from './parser.js';
import { EtaVehicleEvent, EtaVehicleEventTableSchema, rideProjection, ShapeNode, ShapeNodeTableSchema } from './types.js';

/* * */

const RUN_INTERVAL_MS = 60_000 * 60 * 24; // 24 hours
const RIDES_BATCH_SIZE = 30_000;
const VEHICLE_EVENTS_DAYS_CUTOFF = 1;
const AGENCY_IDS = ['41', '42', '43', '44'];
const SHAPE_NODE_CHUNK_LENGTH = 25; // meters
const BATCH_SIZE = 100_000;

/* * */

function getDateRange(): { end: Dates, start: Dates } {
	//

	const now = Dates.now('Europe/Lisbon');
	const start = now.minus({ days: VEHICLE_EVENTS_DAYS_CUTOFF }).set({ hour: 4, minute: 0, second: 0 });
	const endCutoff = now.set({ hour: 4, minute: 0, second: 0 });
	const end = endCutoff.unix_timestamp > now.unix_timestamp
		? now.minus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 })
		: endCutoff;

	//

	return { end, start };
}

async function syncVehicleEvents(client: ReturnType<typeof createClient>, ridesQuery: Filter<Ride>): Promise<{ eventsProcessed: number, ridesProcessed: number }> {
	//

	const writer = new ClickHouseWriter<EtaVehicleEvent>({
		batch_size: BATCH_SIZE,
		client,
		table: 'vehicle_events',
		tableSchema: EtaVehicleEventTableSchema,
	});
	await writer.ensureTable();

	const ridesCollection = await rides.getCollection();
	const vehicleEventsCollection = await simplifiedVehicleEvents.getCollection();

	//
	// Setup Rides Cursor
	const ridesCount = await ridesCollection.countDocuments(ridesQuery);
	Logger.info(`Syncing vehicle events for ${ridesCount} rides (${Dates.fromUnixTimestamp(ridesQuery.start_time_scheduled['$gte']).toFormat('yyyy-MM-dd')} → ${Dates.fromUnixTimestamp(ridesQuery.start_time_scheduled['$lt']).toFormat('yyyy-MM-dd')})`);

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

	await writer.flush();

	return { eventsProcessed, ridesProcessed };
}

async function syncShapeNodes(client: ReturnType<typeof createClient>, ridesQuery: Filter<Ride>): Promise<{ shapeNodesProcessed: number }> {
	//

	const writer = new ClickHouseWriter<ShapeNode>({
		batch_size: BATCH_SIZE,
		client,
		table: 'shape_nodes',
		tableSchema: ShapeNodeTableSchema,
	});
	await client.command({ query: 'DROP TABLE IF EXISTS shape_nodes' });
	await writer.ensureTable();

	Logger.info(`Getting distinct hashed shape ids from rides`);
	const distinctHashedShapeIds = await rides.distinct('hashed_shape_id', ridesQuery);
	const hashedShapesCollection = await hashedShapes.getCollection();

	const hashedShapesCursor = hashedShapesCollection.find(
		{ _id: { $in: distinctHashedShapeIds } },
		{ projection: { _id: 1, points: { shape_pt_lat: 1, shape_pt_lon: 1 } } },
	).batchSize(BATCH_SIZE).stream();

	Logger.info(`Creating shape nodes for ${distinctHashedShapeIds.length} hashed shape ids`);

	const totalShapeNodes = 0;
	for await (const hashedShape of hashedShapesCursor) {
		const geojson = hashedShapesToFeatureCollection(hashedShape);
		const chunks = turf.lineChunk(geojson.features[0], SHAPE_NODE_CHUNK_LENGTH, { units: 'meters' });

		for (const [idx, chunk] of chunks.features.entries()) {
			await writer.write({
				latitude: chunk.geometry.coordinates[0][1],
				longitude: chunk.geometry.coordinates[0][0],
				node_index: idx,
				shape_id: hashedShape._id,
			});
		}
	}

	await writer.flush();

	return { shapeNodesProcessed: totalShapeNodes };
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

	//
	// Get Date Range
	const { end, start } = getDateRange();

	//
	// Setup Rides Query
	const ridesQuery: Filter<Ride> = {
		agency_id: { $in: AGENCY_IDS },
		line_id: { $in: [2652, 2708, 2711, 2713, 2722, 2725, 2728, 2729, 2730, 2731, 2734] }, // ! Development only
		start_time_observed: { $ne: null },
		start_time_scheduled: { $gte: start.unix_timestamp, $lt: end.unix_timestamp },
	};

	//
	// Sync Vehicle Events
	const { eventsProcessed, ridesProcessed } = await syncVehicleEvents(client, ridesQuery);
	Logger.success(`Sync completed: ${ridesProcessed} rides, ${eventsProcessed} events in ${timer.get()}`);

	//
	// Sync Shape Nodes
	const { shapeNodesProcessed } = await syncShapeNodes(client, ridesQuery);
	Logger.success(`Sync completed: ${shapeNodesProcessed} shape nodes`);

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
