/* * */

import type { Coordinate, ShapePoint } from '../types.js';

import * as turf from '@turf/turf';
import { Feature, LineString, Position } from 'geojson';

/* * */

/**
 * Converts hashed shape points to a turf LineString
 *
 * @param points - Array of shape points with lat/lon
 * @returns Turf LineString feature
 */
export function pointsToLineString(points: ShapePoint[]): Feature<LineString> {
	const coordinates = points.map(
		point => [point.shape_pt_lon, point.shape_pt_lat] as Position,
	);
	return turf.lineString(coordinates);
}

/**
 * Chunks a line into segments of specified length and extracts endpoints
 *
 * @param line - Turf LineString to chunk
 * @param segmentLengthMeters - Desired segment length in meters
 * @returns Array of segment endpoint coordinates
 */
export function chunkLineAndExtractEndpoints(
	line: Feature<LineString>,
	segmentLengthMeters: number,
): Coordinate[] {
	const chunks = turf.lineChunk(line, segmentLengthMeters, { units: 'meters' });

	return chunks.features.map((feature) => {
		const coordinates = feature.geometry.coordinates as Position[];
		// Return the last coordinate (endpoint) of each chunk
		return coordinates[coordinates.length - 1] as Coordinate;
	});
}

/**
 * Processes a hashed shape's points into segment endpoints
 * Combines point conversion and chunking into a single operation
 *
 * @param points - Array of shape points
 * @param segmentLengthMeters - Desired segment length in meters
 * @returns Array of segment endpoint coordinates
 */
export function processShapeToSegmentEndpoints(
	points: ShapePoint[],
	segmentLengthMeters: number,
): Coordinate[] {
	const line = pointsToLineString(points);
	return chunkLineAndExtractEndpoints(line, segmentLengthMeters);
}

/* * */
