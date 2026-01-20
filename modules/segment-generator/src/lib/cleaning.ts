/* * */

import type { Coordinate } from '../types.js';

import { latLngToCell } from 'h3-js';

/* * */

/**
 * Calculate perpendicular distance from a point to a line segment
 */
function perpendicularDistance(point: Coordinate, lineStart: Coordinate, lineEnd: Coordinate): number {
	const [px, py] = point;
	const [x1, y1] = lineStart;
	const [x2, y2] = lineEnd;

	// If line segment is a point
	if (x1 === x2 && y1 === y2) {
		return Math.sqrt((px - x1) ** 2 + (py - y1) ** 2);
	}

	let dx = x2 - x1;
	let dy = y2 - y1;
	const length = Math.sqrt(dx ** 2 + dy ** 2);
	dx /= length;
	dy /= length;

	const projX = px - x1;
	const projY = py - y1;
	const projection = projX * dx + projY * dy;

	const closestX = x1 + projection * dx;
	const closestY = y1 + projection * dy;

	return Math.sqrt((px - closestX) ** 2 + (py - closestY) ** 2);
}

/* * */

/**
 * Douglas-Peucker line simplification algorithm
 * Reduces point count while preserving shape
 *
 * @param coords - List of [lon, lat] coordinates
 * @param epsilon - Distance threshold in degrees (~0.00002 ≈ 2m)
 * @returns Simplified coordinates
 */
export function douglasPeucker(coords: Coordinate[], epsilon = 0.00003): Coordinate[] {
	if (coords.length < 3) {
		return coords;
	}

	// Find the point with the maximum distance from the line between start and end
	let maxDist = 0;
	let maxIndex = 0;

	const start = coords[0];
	const end = coords[coords.length - 1];

	for (let i = 1; i < coords.length - 1; i++) {
		const dist = perpendicularDistance(coords[i], start, end);
		if (dist > maxDist) {
			maxDist = dist;
			maxIndex = i;
		}
	}

	// If max distance is greater than epsilon, recursively simplify
	if (maxDist > epsilon) {
		const left = douglasPeucker(coords.slice(0, maxIndex + 1), epsilon);
		const right = douglasPeucker(coords.slice(maxIndex), epsilon);

		// Combine results, excluding duplicate middle point
		return left.slice(0, -1).concat(right);
	}

	// All points are within epsilon, return just start and end
	return [start, end];
}

/* * */

/**
 * Smooth coordinates using a moving average window
 *
 * @param coords - List of [lon, lat] coordinates
 * @param windowSize - Number of points in the window (odd recommended)
 * @returns Smoothed coordinates
 */
export function movingAverageSmooth(coords: Coordinate[], windowSize = 5): Coordinate[] {
	if (coords.length < windowSize) {
		return coords;
	}

	const halfWindow = Math.floor(windowSize / 2);
	const result: Coordinate[] = [...coords]; // Copy array

	for (let i = halfWindow; i < coords.length - halfWindow; i++) {
		const window = coords.slice(i - halfWindow, i + halfWindow + 1);

		let sumLon = 0;
		let sumLat = 0;
		for (const [lon, lat] of window) {
			sumLon += lon;
			sumLat += lat;
		}

		result[i] = [sumLon / window.length, sumLat / window.length];
	}

	return result;
}

/* * */

/**
 * Convert coordinates to H3 hexagon indices
 *
 * @param coords - List of [lon, lat] coordinates (GeoJSON order)
 * @param resolution - H3 resolution (0-15)
 * @returns List of H3 indices
 */
export function coordsToH3(coords: Coordinate[], resolution = 12): string[] {
	const h3Indices: string[] = [];

	for (const [lon, lat] of coords) {
		// H3 expects (lat, lon), GeoJSON provides [lon, lat]
		const h3Index = latLngToCell(lat, lon, resolution);
		h3Indices.push(h3Index);
	}

	return h3Indices;
}

/* * */

/**
 * Majority vote smoothing for H3 indices
 * Replaces each index with the most common value in its window
 *
 * @param h3Indices - List of H3 indices
 * @param windowSize - Size of the voting window
 * @returns Smoothed H3 indices
 */
export function majorityVoteSmooth(h3Indices: string[], windowSize = 5): string[] {
	if (h3Indices.length < windowSize) {
		return h3Indices;
	}

	const halfWindow = Math.floor(windowSize / 2);
	const result: string[] = [];

	for (let i = 0; i < h3Indices.length; i++) {
		const start = Math.max(0, i - halfWindow);
		const end = Math.min(h3Indices.length, i + halfWindow + 1);
		const window = h3Indices.slice(start, end);

		// Count occurrences and find the most common
		const counts = new Map<string, number>();
		for (const idx of window) {
			counts.set(idx, (counts.get(idx) || 0) + 1);
		}

		// Find the most common
		let maxCount = 0;
		let majority = h3Indices[i];
		for (const [idx, count] of counts) {
			if (count > maxCount) {
				maxCount = count;
				majority = idx;
			}
		}

		result.push(majority);
	}

	return result;
}

/* * */

/**
 * Remove hexagons that appear fewer than minConsecutive times in a row
 *
 * @param h3Indices - List of H3 indices
 * @param minConsecutive - Minimum consecutive occurrences to keep
 * @returns Filtered H3 indices (one per group that meets threshold)
 */
export function removeTransientHexagons(h3Indices: string[], minConsecutive = 2): string[] {
	if (h3Indices.length === 0) {
		return [];
	}

	const groups: [string, number][] = [];
	let currentHex = h3Indices[0];
	let count = 1;

	for (let i = 1; i < h3Indices.length; i++) {
		if (h3Indices[i] === currentHex) {
			count++;
		}
		else {
			groups.push([currentHex, count]);
			currentHex = h3Indices[i];
			count = 1;
		}
	}
	groups.push([currentHex, count]);

	// Return one hexagon per group that meets threshold
	return groups.filter(([, cnt]) => cnt >= minConsecutive).map(([hexId]) => hexId);
}

/* * */

/**
 * Detect and remove loops (GPS drift) where the path revisits a recent hexagon
 *
 * @param h3Indices - List of H3 indices
 * @param maxLookback - Maximum steps to look back for loops
 * @returns H3 indices with loops removed
 */
export function detectLoopsAndBacktracking(h3Indices: string[], maxLookback = 5): string[] {
	if (h3Indices.length < 3) {
		return h3Indices;
	}

	const cleaned: string[] = [];
	let i = 0;

	while (i < h3Indices.length) {
		const current = h3Indices[i];

		// Check if this hexagon appears again soon (indicating a loop)
		let loopEnd = -1;
		for (let j = i + 2; j < Math.min(i + maxLookback + 1, h3Indices.length); j++) {
			if (h3Indices[j] === current) {
				loopEnd = j;
				break;
			}
		}

		if (loopEnd > 0) {
			cleaned.push(current);
			i = loopEnd + 1;
		}
		else {
			cleaned.push(current);
			i++;
		}
	}

	return cleaned;
}

/* * */

/**
 * Remove duplicate hexagons from the path
 *
 * @param h3Indices - List of H3 indices
 * @param removeAll - If true, remove all duplicates keeping first occurrence.
 *                    If false, only remove consecutive duplicates.
 * @returns Deduplicated H3 indices
 */
export function deduplicateH3Path(h3Indices: string[], removeAll = true): string[] {
	if (h3Indices.length === 0) {
		return h3Indices;
	}

	if (removeAll) {
		// Remove all duplicates, keeping first occurrence
		const seen = new Set<string>();
		const result: string[] = [];

		for (const idx of h3Indices) {
			if (!seen.has(idx)) {
				seen.add(idx);
				result.push(idx);
			}
		}

		return result;
	}
	else {
		// Only remove consecutive duplicates
		const result: string[] = [h3Indices[0]];

		for (let i = 1; i < h3Indices.length; i++) {
			if (h3Indices[i] !== h3Indices[i - 1]) {
				result.push(h3Indices[i]);
			}
		}

		return result;
	}
}

/* * */
