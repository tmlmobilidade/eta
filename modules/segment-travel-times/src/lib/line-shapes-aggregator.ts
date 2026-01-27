/* * */

import type { Coordinate, HashedShapePointProjection, LineShapeData, LineShapesMap, RideProjection, SegmentTravelTimesSettings } from '../types.js';

import { Logger } from '@tmlmobilidade/logger';

import { encodeCoordinate } from '../utils/geohash-utils.js';
import { fetchHashedShapesByIds } from './hashed-shapes-service.js';
import { processShapeToSegmentEndpoints } from './segment-processor.js';

/* * */

/**
 * Creates an empty LineShapeData structure
 *
 * @returns New LineShapeData with initialized collections
 */
function createEmptyLineShapeData(): LineShapeData {
	return {
		geohashes: new Set(),
		hashedShapeIds: [],
		nodes: new Map(),
	};
}

/**
 * Encodes segment endpoints to geohashes and adds them to the line data
 *
 * @param lineData - The line shape data to update
 * @param endpoints - Array of segment endpoint coordinates
 * @param geohashPrecision - Geohash precision level
 */
function addGeohashesFromEndpoints(lineData: LineShapeData, endpoints: Coordinate[], geohashPrecision: number): void {
	for (const endpoint of endpoints) {
		const hash = encodeCoordinate(endpoint, geohashPrecision);
		lineData.geohashes.add(hash);
	}
}

/**
 * Processes a single ride using pre-fetched hashed shape data
 *
 * @param ride - The ride document projection
 * @param hashedShape - The pre-fetched hashed shape data
 * @param lineShapes - Map to update with line shape data
 * @param settings - Settings containing geohash precision and segment length
 */
function processRideWithHashedShape(ride: RideProjection, hashedShape: HashedShapePointProjection, lineShapes: LineShapesMap, settings: SegmentTravelTimesSettings): void {
	// Process shape into segment endpoints
	const segmentEndpoints = processShapeToSegmentEndpoints(
		hashedShape.points,
		settings.segmentLengthMeters,
	);

	// Get or create line data entry
	let lineData = lineShapes.get(ride.line_id);
	if (!lineData) {
		lineData = createEmptyLineShapeData();
		lineShapes.set(ride.line_id, lineData);
	}

	// Update line data
	lineData.hashedShapeIds.push(ride.hashed_shape_id);
	lineData.nodes.set(ride.hashed_shape_id, segmentEndpoints);
	addGeohashesFromEndpoints(lineData, segmentEndpoints, settings.geohashPrecision);
}

/**
 * Aggregates all rides into line shapes map
 * Processes rides cursor and groups shapes by line_id
 *
 * Optimized to batch fetch all hashed shapes in a single database call
 *
 * @param cursor - Async iterable cursor of ride documents
 * @param settings - Settings for processing
 * @returns Object containing the line shapes map and count of processed shapes
 */
export async function aggregateRidesToLineShapes(cursor: AsyncIterable<RideProjection>, settings: SegmentTravelTimesSettings): Promise<{ lineShapes: LineShapesMap, processedCount: number }> {
	// Step 1: Collect all rides and unique hashed_shape_ids
	const rides: RideProjection[] = [];
	const uniqueHashedShapeIds = new Set<string>();

	for await (const ride of cursor) {
		rides.push(ride);
		uniqueHashedShapeIds.add(ride.hashed_shape_id);
	}

	Logger.info(`Collected ${rides.length} rides with ${uniqueHashedShapeIds.size} unique hashed shapes`);

	// Step 2: Batch fetch all hashed shapes in a single database call
	const hashedShapesMap = await fetchHashedShapesByIds(Array.from(uniqueHashedShapeIds));

	Logger.info(`Fetched ${hashedShapesMap.size} hashed shapes from database`);

	// Step 3: Process rides using pre-fetched hashed shapes
	const processedHashedShapeIds = new Set<string>();
	const lineShapes: LineShapesMap = new Map();

	for (const ride of rides) {
		// Skip if this hashed_shape_id was already processed
		if (processedHashedShapeIds.has(ride.hashed_shape_id)) {
			continue;
		}

		const hashedShape = hashedShapesMap.get(ride.hashed_shape_id);
		if (!hashedShape) {
			continue;
		}

		processedHashedShapeIds.add(ride.hashed_shape_id);
		processRideWithHashedShape(ride, hashedShape, lineShapes, settings);
	}

	Logger.info(`Grouped ${processedHashedShapeIds.size} hashed shapes into ${lineShapes.size} lines`);

	return { lineShapes, processedCount: processedHashedShapeIds.size };
}

/* * */
