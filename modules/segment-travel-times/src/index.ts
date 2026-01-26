/* * */

import { createClient } from '@clickhouse/client';
import { Dates } from '@tmlmobilidade/dates';
import { AggregationCursor, AggregationPipeline, hashedShapes, rides } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { ClickHouseVehicleEvent, Ride } from '@tmlmobilidade/types';
import * as turf from '@turf/turf';
import { Position } from 'geojson';
import geohash from 'ngeohash';

import { Coordinate } from './types.js';

/* * */

const RUN_INTERVAL = 60_000 * 10; // 10 minutes in milliseconds

const settings = {
	geohashPrecision: 7,
	rideEndDate: Dates.now('Europe/Lisbon').set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
	rideStartDate: Dates.now('Europe/Lisbon').minus({ days: 7 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
	segmentLengthMeters: 50,
};

async function main() {
	//

	Logger.init();

	const globalTimer = new Timer();
	const clickhouseClient = createClient({
		database: process.env.CLICKHOUSE_DATABASE,
		password: process.env.CLICKHOUSE_PASSWORD,
		url: `http://${process.env.CLICKHOUSE_HOST}:${process.env.CLICKHOUSE_PORT}`,
		username: process.env.CLICKHOUSE_USERNAME,
	});

	// 1. Fetch rides from the database
	const aggregationPipeline: AggregationPipeline<Ride> = [
		{ $match: { start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate } } },
		{
			$project: {
				_id: 1,
				hashed_shape_id: 1,
				line_id: 1,
				start_time_scheduled: 1,
				trip_id: 1,
				vehicle_ids: 1,
			},
		},
	];
	const collection = await rides.getCollection();
	const cursor = collection.aggregate<Ride>(aggregationPipeline).batchSize(10_000);
	const totalRides = await collection.countDocuments({
		start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate },
	});

	// 1.2. Fetch all hashed shapes and chunk them into segments, grouped by line_id
	Logger.info(`Fetching ${totalRides} rides...`);
	const processedHashedShapeIds = new Set<string>();
	const lineShapes = new Map<number, { geohashes: Set<string>, hashedShapeIds: string[], nodes: Map<string, Coordinate[]> }>();

	for await (const doc of cursor) {
		// Skip if the hashed shape id has already been processed
		if (processedHashedShapeIds.has(doc.hashed_shape_id)) continue;
		processedHashedShapeIds.add(doc.hashed_shape_id);

		// Fetch the hashed shape with its points
		const hashed_shape = await hashedShapes.findOne(
			{ _id: doc.hashed_shape_id },
			{ projection: { _id: 1, points: { shape_pt_lat: 1, shape_pt_lon: 1 } } },
		);
		if (!hashed_shape) continue;

		// Convert points to GeoJSON LineString format [longitude, latitude]
		const line = turf.lineString(
			hashed_shape.points.map(point => [point.shape_pt_lon, point.shape_pt_lat] as Position),
		);

		// Chunk the line into segments of specified length
		const chunks = turf.lineChunk(line, settings.segmentLengthMeters, { units: 'meters' });

		// Extract start and end coordinates for each segment chunk
		// Each chunk is a LineString with multiple coordinates, we need the endpoints
		const segmentEndpoints: Coordinate[] = chunks.features.map((feature) => {
			const coordinates = feature.geometry.coordinates as Position[];
			// Return the last coordinate (endpoint) of each chunk as the segment endpoint
			return coordinates[coordinates.length - 1] as Coordinate;
		});

		// Get or create the line entry and add this hashed shape's data
		let lineData = lineShapes.get(doc.line_id);
		if (!lineData) {
			lineData = { geohashes: new Set(), hashedShapeIds: [], nodes: new Map() };
			lineShapes.set(doc.line_id, lineData);
		}

		lineData.hashedShapeIds.push(doc.hashed_shape_id);
		lineData.nodes.set(doc.hashed_shape_id, segmentEndpoints);
		for (const endpoint of segmentEndpoints) {
			lineData.geohashes.add(geohash.encode(endpoint[1], endpoint[0], settings.geohashPrecision));
		}
	}

	Logger.info(`Grouped ${processedHashedShapeIds.size} hashed shapes into ${lineShapes.size} lines`);

	// Cache for vehicle events by geohash - persists across lines
	const geohashEventsCache = new Map<string, ClickHouseVehicleEvent[]>();

	// 2. Fetch vehicle events grouped by line_id
	for (const [lineId, { geohashes, hashedShapeIds, nodes }] of Array.from(lineShapes.entries()).sort(([aId], [bId]) => aId - bId)) {
		Logger.title(`Processing line ${lineId} with ${hashedShapeIds.length} hashed shapes and ${geohashes.size} geohashes`);

		// 2.1 Find geohashes that haven't been cached yet
		const allGeohashes = Array.from(geohashes);
		const uncachedGeohashes = allGeohashes.filter(gh => !geohashEventsCache.has(gh));
		const cachedGeohashes = allGeohashes.filter(gh => geohashEventsCache.has(gh));

		Logger.info(`Found ${cachedGeohashes.length} cached geohashes, ${uncachedGeohashes.length} to fetch`);

		// 2.2 Fetch only uncached geohashes from ClickHouse
		if (uncachedGeohashes.length > 0) {
			const query = `
				WHERE created_at >= ${settings.rideStartDate} AND created_at < ${settings.rideEndDate}
				AND geohash_${settings.geohashPrecision} IN ('${uncachedGeohashes.join('\',\'')}')
			`;

			const result = await clickhouseClient.query({
				format: 'JSONEachRow',
				query: `SELECT trip_id, geohash_${settings.geohashPrecision} as geohash FROM vehicle_events ${query}`,
			});

			// Initialize cache entries for uncached geohashes
			for (const gh of uncachedGeohashes) {
				geohashEventsCache.set(gh, []);
			}

			let currentRow = 0;
			for await (const rows of result.stream()) {
				for (const row of rows) {
					currentRow++;
					if (currentRow % 100_000 === 0) {
						Logger.info(`Fetched ${currentRow} rows...`);
					}
					const data = row.json<ClickHouseVehicleEvent & { geohash: string }>();
					const cachedEvents = geohashEventsCache.get(data.geohash);
					if (cachedEvents) {
						cachedEvents.push(data);
					}
				}
			}
			Logger.info(`Fetched and cached ${currentRow} events for ${uncachedGeohashes.length} geohashes`);
		}

		// 2.3 Aggregate events from cache for all geohashes in this line
		const groupedEvents = new Map<string, number>();
		for (const gh of allGeohashes) {
			const events = geohashEventsCache.get(gh) ?? [];
			for (const event of events) {
				groupedEvents.set(event.trip_id, (groupedEvents.get(event.trip_id) ?? 0) + 1);
			}
		}

		console.log(groupedEvents);

		Logger.divider();
	}

	Logger.terminate(`Terminated in ${globalTimer.get()}`);

	//
}

/* * */

(async function init() {
	const runOnInterval = async () => {
		await main();
		setTimeout(runOnInterval, RUN_INTERVAL);
	};
	runOnInterval();
})();
