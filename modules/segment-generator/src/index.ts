/* * */

import { Dates } from '@tmlmobilidade/dates';
import { AggregationPipeline, hashedShapes, rides } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { HashedShape, Ride } from '@tmlmobilidade/types';
import * as turf from '@turf/turf';
import { createWriteStream } from 'fs';
import { FeatureCollection, LineString } from 'geojson';

import { coordsToH3, deduplicateH3Path, detectLoopsAndBacktracking, douglasPeucker, majorityVoteSmooth, movingAverageSmooth, removeTransientHexagons } from './lib/cleaning.js';
import { generateHexagonsGeoJSON } from './lib/geojsonOutput.js';
import { hashedShapesToFeatureCollection } from './lib/hashed-shapes-to-feature.js';
import { filterGeoJSONByClusterSize, filterGeoJSONByMaxSize } from './lib/postProcessing.js';
import { Coordinate, DEFAULT_CONFIG } from './types.js';

/* * */

const RUN_INTERVAL = 60_000 * 60 * 24; // 1 day in milliseconds

const settings = {
	rideEndDate: Dates.now('Europe/Lisbon').plus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
	rideStartDate: Dates.now('Europe/Lisbon').minus({ days: 7 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
};

/**
 * Streaming GeoJSON writer that writes features incrementally to avoid memory issues
 */
class StreamingGeoJSONWriter {
	private isClosed = false;
	private isFirstFeature = true;
	private writeStream: ReturnType<typeof createWriteStream>;

	constructor(filePath: string) {
		this.writeStream = createWriteStream(filePath);
		this.writeStream.write('{\n  "type": "FeatureCollection",\n  "features": [\n');
	}

	async close(): Promise<void> {
		if (this.isClosed) {
			return;
		}
		this.isClosed = true;

		return new Promise<void>((resolve, reject) => {
			this.writeStream.write('\n  ]\n}');
			this.writeStream.end();

			this.writeStream.on('finish', resolve);
			this.writeStream.on('error', reject);
		});
	}

	writeFeature(feature: GeoJSON.Feature): void {
		if (this.isClosed) {
			throw new Error('StreamingGeoJSONWriter is already closed');
		}

		if (!this.isFirstFeature) {
			this.writeStream.write(',\n');
		}
		this.isFirstFeature = false;

		const featureJson = JSON.stringify(feature, null, 2)
			.split('\n')
			.map(line => '    ' + line)
			.join('\n');

		this.writeStream.write(featureJson);
	}
}

/**
 * Write a GeoJSON FeatureCollection to a file incrementally to avoid "Invalid string length" errors
 */
async function writeGeoJSONFile(filePath: string, featureCollection: FeatureCollection): Promise<void> {
	const writer = new StreamingGeoJSONWriter(filePath);
	for (const feature of featureCollection.features) {
		writer.writeFeature(feature);
	}
	await writer.close();
}

async function main() {
	//

	Logger.init();

	const globalTimer = new Timer();

	//
	// 1. Get All Unique Hashed Shape IDs
	Logger.info('Getting all unique hashed shape IDs');

	const aggregationPipeline: AggregationPipeline<Ride> = [
		{
			$match: {
				start_time_scheduled: {
					$gte: settings.rideStartDate,
					$lt: settings.rideEndDate,
				},
			},
		},
		// {
		// 	$match: {
		// 		line_id: { $in: [2711, 2725, 2730, 2708, 2728, 2722] },
		// 	},
		// },
		{
			$group: {
				_id: null,
				hashed_shape_ids: { $addToSet: '$hashed_shape_id' },
			},
		},
		{
			$project: {
				_id: 0,
				hashed_shape_ids: 1,
			},
		},
	];
	const result = (await rides.aggregate(aggregationPipeline)) as unknown as { hashed_shape_ids: string[] }[];
	const hashedShapeIds = result[0]?.hashed_shape_ids ?? [];
	Logger.info(`Found ${hashedShapeIds.length} unique hashed shape IDs`);

	// 2. Get All Coordinates from all hashed shapes and stream features to file
	const allCoordinates: Coordinate[] = [];
	const collection = await hashedShapes.getCollection();
	const cursor = collection.find({ _id: { $in: hashedShapeIds } }).batchSize(10_000);
	const totalDocuments = await collection.countDocuments({ _id: { $in: hashedShapeIds } });
	let currentDocument = 0;

	// Stream features directly to file to avoid memory issues
	const geoJsonWriter = new StreamingGeoJSONWriter('hashedShapes.geojson');

	console.log(`Processing ${totalDocuments} documents...`);
	for await (const doc of cursor) {
		currentDocument++;
		console.log(`Processing document ${currentDocument} of ${totalDocuments}...`);
		const hashedShapesFeatureCollection: GeoJSON.FeatureCollection<GeoJSON.Geometry> = hashedShapesToFeatureCollection(doc);

		for (const feature of hashedShapesFeatureCollection.features) {
			const chunks = turf.lineChunk(feature.geometry as LineString, 5, { units: 'meters' });
			for (const chunkFeature of chunks.features) {
				// Write feature directly to file instead of accumulating in memory
				geoJsonWriter.writeFeature(chunkFeature);
				allCoordinates.push(...(chunkFeature.geometry.coordinates as Coordinate[]));
			}
		}
	}

	// Close the streaming writer
	await geoJsonWriter.close();

	// 3. Clean Coordinates
	Logger.title('H3 LINE CLEANING PIPELINE');
	Logger.info(`Original points: ${allCoordinates.length}`);

	// Convert to H3 indices
	const h3Raw = coordsToH3(allCoordinates, DEFAULT_CONFIG.h3Resolution);
	const uniqueRaw = new Set(h3Raw).size;

	// Clean at this resolution
	const h3Voted = majorityVoteSmooth(h3Raw, DEFAULT_CONFIG.voteWindow);
	const h3NoTransient = removeTransientHexagons(h3Voted, DEFAULT_CONFIG.minConsecutive);
	const h3NoLoops = detectLoopsAndBacktracking(h3NoTransient, DEFAULT_CONFIG.maxLookback);
	const beforeDedup = h3NoLoops.length;
	const h3Final = deduplicateH3Path(h3NoLoops, true);
	const duplicatesRemoved = beforeDedup - h3Final.length;

	Logger.info(`  Resolution ${DEFAULT_CONFIG.h3Resolution}: ${uniqueRaw} raw → ${h3Final.length} cleaned (removed ${duplicatesRemoved} duplicates)`);

	// 4. Segment the path
	const segments = new Map<number, string[]>();
	for (let i = 0; i < h3Final.length; i++) {
		const segIdx = Math.floor(i / DEFAULT_CONFIG.segmentSize);
		if (!segments.has(segIdx)) {
			segments.set(segIdx, []);
		}
		const segment = segments.get(segIdx);
		if (segment) segment.push(h3Final[i]);
	}

	const hexFeatures: GeoJSON.Feature<GeoJSON.Geometry>[] = [];
	for (const [segIdx, segH3] of segments) {
		const features: GeoJSON.Feature<GeoJSON.Geometry>[] = generateHexagonsGeoJSON(segH3, segIdx);
		hexFeatures.push(...features);
	}

	let hexGeoJSON: GeoJSON.FeatureCollection = { features: hexFeatures, type: 'FeatureCollection' };

	console.log('hexGeoJSON', hexGeoJSON);
	// Apply cluster filtering if min_size > 1
	if (DEFAULT_CONFIG.minSize > 1 && DEFAULT_CONFIG.segmentSize) {
		hexGeoJSON = filterGeoJSONByClusterSize(hexGeoJSON, DEFAULT_CONFIG.minSize);
	}
	// Apply max-size splitting if specified
	if (DEFAULT_CONFIG.segmentSize) {
		hexGeoJSON = filterGeoJSONByMaxSize(hexGeoJSON, DEFAULT_CONFIG.segmentSize);
	}

	// Save outputs
	await writeGeoJSONFile('hexagons.geojson', hexGeoJSON);

	Logger.title('SUMMARY');
	Logger.info(`Total H3 hexagons processed: ${h3Final.length}`);
	Logger.info(`Hexagons in output: ${hexGeoJSON.features.length}`);

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
