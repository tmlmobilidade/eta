/* * */

import { Dates } from '@tmlmobilidade/dates';
import { AggregationPipeline, hashedShapes, rides, simplifiedVehicleEvents } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { Ride, SimplifiedVehicleEvent } from '@tmlmobilidade/types';
import * as turf from '@turf/turf';
import { Position } from 'geojson';
import geohash from 'ngeohash';

import { Coordinate } from './types.js';
import { geohashRange } from './utils/geohashRange.js';

/* * */

const RUN_INTERVAL = 60_000; // 1 minute in milliseconds

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

	// 1. Fetch rides from the database
	const aggregationPipeline: AggregationPipeline<Ride> = [
		{ $match: { start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate } } },
		{ $match: { line_id: { $in: [2730] } } },
		{
			$project: {
				_id: 1,
				hashed_shape_id: 1,
				start_time_scheduled: 1,
				trip_id: 1,
				vehicle_ids: 1,
			},
		},
	];
	const collection = await rides.getCollection();
	const cursor = collection.aggregate(aggregationPipeline).batchSize(10_000);
	const totalRides = await collection.countDocuments({
		line_id: { $in: [1001] },
		start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate },
	});

	// 1.2. Fetch all hashed shapes and chunk them into segments
	Logger.info(`Fetching ${totalRides} rides...`);
	const hashedShapeIds = new Map<string, { geohashes: Set<string>, node: Coordinate[] }>();
	for await (const doc of cursor) {
		// Skip if the hashed shape id has already been processed
		if (hashedShapeIds.has(doc.hashed_shape_id)) continue;

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

		hashedShapeIds.set(doc.hashed_shape_id, { geohashes: new Set(segmentEndpoints.map(endpoint => geohash.encode(endpoint[1], endpoint[0], settings.geohashPrecision))), node: segmentEndpoints });
	}

	// 2. Test Vehicle Events for each hashed shape
	for (const [hashedShapeId, { geohashes, node }] of hashedShapeIds.entries()) {
		Logger.title(`Testing hashed shape ${hashedShapeId} with ${geohashes.size} geohashes and ${node.length} nodes`);

		// 2.1 Fetch all vehicle events for the hashed shape
		const aggregationPipeline = [
			{ $match: { created_at: { $gte: settings.rideStartDate, $lt: settings.rideEndDate } } },
			{ $match: geohashRange(Array.from(geohashes)) },
		];
		const collection = await simplifiedVehicleEvents.getCollection();
		const cursor = collection.aggregate(aggregationPipeline).batchSize(100_000);

		const groupedEvents = new Map<string, number>();
		let currentDocument = 0;
		for await (const doc of cursor) {
			currentDocument++;
			if (currentDocument % 5000 === 0) {
				Logger.info(`Processing document ${currentDocument}...`);
			}
			const ve = doc as SimplifiedVehicleEvent;

			const operationalDate = Dates.fromUnixTimestamp(ve.created_at).setZone('Europe/Lisbon', 'rebase_utc').operational_date;
			groupedEvents.set(ve.trip_id + '-' + operationalDate, (groupedEvents.get(ve.trip_id + '-' + operationalDate) ?? 0) + 1);
		}

		for (const [tripIdOperationalDate, count] of groupedEvents.entries()) {
			const [tripId, operationalDate] = tripIdOperationalDate.split('-');
			Logger.info(`${tripId} - ${operationalDate}: ${count}`);
		}

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
