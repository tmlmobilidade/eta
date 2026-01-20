/* * */

import { cellToBoundary, cellToLatLng } from 'h3-js';

import { type Coordinate, DEFAULT_CONFIG } from '../types.js';
import { removeIsolatedHexagons } from './postProcessing.js';

/* * */

const AVAILABLE_COLORS = [
	'#0066FF', // vivid blue
	'#FF3B30', // vivid red
	'#34C759', // vivid green
	'#FF9500', // vivid orange
	'#AF52DE', // vivid purple
	'#00C7BE', // vivid teal
	'#FF2D55', // vivid pink
	'#FFD60A', // vivid yellow
	'#32ADE6', // vivid cyan
	'#FF6B00', // vivid deep orange
	'#5856D6', // vivid indigo
	'#30D158', // neon green
	'#FF453A', // coral red
	'#64D2FF', // sky blue
	'#BF5AF2', // bright violet
	'#5E5CE6', // electric blue
	'#FF9F0A', // amber
	'#40C8E0', // aqua
	'#FF375F', // hot pink
	'#7D7AFF', // periwinkle
];

/**
 * Generate a color for an index using HSL color space
 *
 * @param index - Current index
 * @param total - Total number of items
 * @param saturation - Color saturation (0-1)
 * @param lightness - Color lightness (0-1)
 * @returns Hex color string
 */
export function getColorForIndex(
	index: number,
): string {
	return AVAILABLE_COLORS[index % AVAILABLE_COLORS.length];
}

/* * */

/**
 * Create a GeoJSON feature for a single H3 hexagon
 *
 * @param h3Index - H3 cell identifier
 * @param order - Position in sequence
 * @param color - Fill and stroke color
 * @param extraProps - Additional properties to include
 * @returns GeoJSON Feature with Polygon geometry
 */
export function createHexagonFeature(
	h3Index: string,
	order: number,
	color: string,
	extraProps: Record<string, unknown> = {},
): GeoJSON.Feature<GeoJSON.Polygon> {
	// Get the boundary coordinates from H3
	// cellToBoundary returns [[lat, lon], ...], we need [[lon, lat], ...] for GeoJSON
	const boundary = cellToBoundary(h3Index);
	const coordinates: Coordinate[] = boundary.map(([lat, lon]) => [lon, lat]);

	// Close the polygon (GeoJSON requires first point = last point)
	coordinates.push(coordinates[0]);

	const properties: Record<string, unknown> = {
		'fill': color,
		'fill-opacity': 0.6,
		'group_id': null,
		'group_size': 0,
		'h3_index': h3Index,
		order,
		'segment_index': null,
		'stroke': color,
		'stroke-width': 1,
		...extraProps,
	};

	return {
		geometry: {
			coordinates: [coordinates],
			type: 'Polygon',
		},
		properties,
		type: 'Feature',
	};
}

/* * */

/**
 * Create a GeoJSON LineString feature
 *
 * @param coords - Array of [lon, lat] coordinates
 * @param lineType - Type identifier for the line
 * @param color - Stroke color
 * @param width - Stroke width
 * @param chunkIndex - Chunk index for chunked processing
 * @returns GeoJSON Feature with LineString geometry
 */
export function createLineFeature(
	coords: Coordinate[],
	lineType: string,
	color: string,
	width: number,
	chunkIndex = 0,
): GeoJSON.Feature<GeoJSON.LineString> {
	return {
		geometry: {
			coordinates: coords,
			type: 'LineString',
		},
		properties: {
			'chunk_index': chunkIndex,
			'stroke': color,
			'stroke-width': width,
			'type': lineType,
		},
		type: 'Feature',
	};
}

/* * */

/**
 * Generate GeoJSON features for visualization
 *
 * @param originalCoords - Original input coordinates
 * @param cleanedCoords - Coordinates after cleaning
 * @param h3Indices - H3 hexagon indices
 * @param chunkIndex - Chunk index for chunked processing
 * @returns Array of GeoJSON features
 */
export function generateVisualizationGeoJSON(
	originalCoords: Coordinate[],
	cleanedCoords: Coordinate[],
	h3Indices: string[],
	chunkIndex = 0,
): GeoJSON.Feature<GeoJSON.Geometry>[] {
	const features: GeoJSON.Feature<GeoJSON.Geometry>[] = [];

	// Original line (blue)
	if (originalCoords.length >= 2) {
		features.push(createLineFeature(originalCoords, 'original', '#3388ff', 2, chunkIndex));
	}

	// Cleaned line (green)
	if (cleanedCoords.length >= 2) {
		features.push(createLineFeature(cleanedCoords, 'cleaned', '#33ff88', 3, chunkIndex));
	}

	// H3 path (orange line connecting centers)
	if (h3Indices.length > 0) {
		const centers: Coordinate[] = h3Indices.map((h) => {
			const [lat, lon] = cellToLatLng(h);
			return [lon, lat];
		});

		if (centers.length >= 2) {
			features.push(createLineFeature(centers, 'h3_path', '#ff8833', 4, chunkIndex));
		}

		// H3 hexagon boundaries
		for (let i = 0; i < h3Indices.length; i++) {
			features.push(createHexagonFeature(
				h3Indices[i],
				i,
				'#ff8833',
				{
					'chunk_index': chunkIndex,
					'fill-opacity': 0.3,
					'type': 'h3_cell',
				},
			));
		}
	}

	return features;
}

/* * */

/**
 * Generate GeoJSON features for H3 hexagons
 *
 * @param h3Indices - List of H3 indices
 * @param config - Cleaning configuration
 * @param segmentIndex - Segment/chunk identifier
 * @param extraProps - Additional properties to add to each hexagon
 * @returns Array of GeoJSON features
 */
export function generateHexagonsGeoJSON(
	h3Indices: string[],
	segmentIndex: null | number = null,
	extraProps: Record<string, unknown> = {},
): GeoJSON.Feature<GeoJSON.Geometry>[] {
	if (h3Indices.length === 0) {
		return [];
	}

	// Remove isolated hexagons if configured
	let processedIndices = h3Indices;
	if (h3Indices.length > 1) {
		processedIndices = removeIsolatedHexagons(h3Indices);
	}

	// Filter by minimum size
	if (processedIndices.length < DEFAULT_CONFIG.minSize) {
		return [];
	}

	// Determine color
	const color = segmentIndex !== null ? getColorForIndex(segmentIndex) : AVAILABLE_COLORS[0];

	// Generate group_id
	const groupId = segmentIndex !== null ? `group_${segmentIndex}` : null;

	const features: GeoJSON.Feature<GeoJSON.Geometry>[] = [];
	for (let i = 0; i < processedIndices.length; i++) {
		features.push(createHexagonFeature(
			processedIndices[i],
			i,
			color,
			{
				group_id: groupId,
				group_size: processedIndices.length,
				segment_index: segmentIndex,
				...extraProps,
			},
		));
	}

	console.log(`Generated ${features.length} features for segment ${segmentIndex}`);

	return features;
}

/* * */

/**
 * Generate full visualization GeoJSON FeatureCollection
 *
 * @param originalCoords - Original input coordinates
 * @param smoothedCoords - Coordinates after simplification/smoothing
 * @param h3Indices - Final H3 indices
 * @param chunkIndex - Chunk index for chunked processing
 * @returns GeoJSON FeatureCollection
 */
export function generateFullVisualizationGeoJSON(
	originalCoords: Coordinate[],
	smoothedCoords: Coordinate[],
	h3Indices: string[],
	chunkIndex = 0,
): GeoJSON.FeatureCollection<GeoJSON.Geometry> {
	const features = generateVisualizationGeoJSON(originalCoords, smoothedCoords, h3Indices, chunkIndex);

	return {
		features,
		type: 'FeatureCollection',
	};
}

/* * */
