/**
 * A coordinate pair [longitude, latitude] in GeoJSON order
 */
export type Coordinate = [number, number];

/**
 * Default configuration values
 */
export const DEFAULT_CONFIG = {
	dpEpsilon: 0.00002,
	h3Resolution: 12,
	maxLookback: 3,
	maxSize: 30,
	minConsecutive: 2,
	minSize: 3,
	removeIsolated: true,
	segmentSize: 30,
	smoothingWindow: 3,
	voteWindow: 5,
};
