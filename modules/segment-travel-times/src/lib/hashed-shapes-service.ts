/* * */

import type { HashedShapePointProjection } from '../types.js';

import { hashedShapes } from '@tmlmobilidade/interfaces';

/* * */

/**
 * Fetches multiple hashed shapes by their IDs in a single database call
 *
 * @param hashedShapeIds - Array of hashed shape document IDs
 * @returns Map of hashed shape ID to its data
 */
export async function fetchHashedShapesByIds(hashedShapeIds: string[]): Promise<Map<string, HashedShapePointProjection>> {
	const collection = await hashedShapes.getCollection();
	const results = await collection.find(
		{ _id: { $in: hashedShapeIds } },
		{ projection: { _id: 1, points: { shape_pt_lat: 1, shape_pt_lon: 1 } } },
	).toArray() as HashedShapePointProjection[];

	const map = new Map<string, HashedShapePointProjection>();
	for (const shape of results) {
		map.set(shape._id, shape);
	}
	return map;
}

/* * */
