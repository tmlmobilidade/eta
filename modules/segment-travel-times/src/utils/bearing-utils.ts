/* * */

import type { Coordinate } from '../types.js';

import * as turf from '@turf/turf';

/* * */

/**
 * Calculates the bearing (direction) from one coordinate to another
 *
 * @param from - Starting coordinate [longitude, latitude]
 * @param to - Ending coordinate [longitude, latitude]
 * @returns Bearing in degrees (0-360, where 0 is North)
 */
export function calculateBearing(from: Coordinate, to: Coordinate): number {
	const bearing = turf.bearing(turf.point(from), turf.point(to));
	// turf.bearing returns -180 to 180, normalize to 0-360
	return bearing < 0 ? bearing + 360 : bearing;
}

/**
 * Calculates the absolute angular difference between two bearings
 * Handles wraparound at 0/360 degrees
 *
 * @param bearing1 - First bearing in degrees (0-360)
 * @param bearing2 - Second bearing in degrees (0-360)
 * @returns Absolute difference in degrees (0-180)
 */
export function getAngularDifference(bearing1: number, bearing2: number): number {
	let diff = Math.abs(bearing1 - bearing2);
	// Handle wraparound: if diff > 180, the shorter angle is 360 - diff
	if (diff > 180) {
		diff = 360 - diff;
	}
	return diff;
}

/**
 * Checks if an event bearing is valid (matches shape direction within threshold)
 * An event is valid if its bearing is within the threshold of the shape bearing
 *
 * @param eventBearing - Bearing of the vehicle event in degrees
 * @param shapeBearing - Expected bearing from the shape in degrees
 * @param thresholdDegrees - Maximum allowed difference in degrees
 * @returns True if the event bearing is valid (within threshold)
 */
export function isValidBearing(eventBearing: number, shapeBearing: number, thresholdDegrees: number): boolean {
	const difference = getAngularDifference(eventBearing, shapeBearing);
	return difference <= thresholdDegrees;
}

/**
 * Calculates bearings for consecutive shape nodes
 * Returns an array where index i contains the bearing from node[i] to node[i+1]
 * The last node has no bearing (set to NaN)
 *
 * @param nodes - Array of node coordinates
 * @returns Array of bearings for each node pair
 */
export function calculateShapeBearings(nodes: Coordinate[]): number[] {
	if (nodes.length < 2) {
		return [];
	}

	const bearings: number[] = [];
	for (let i = 0; i < nodes.length - 1; i++) {
		bearings.push(calculateBearing(nodes[i], nodes[i + 1]));
	}
	// Last node doesn't have a "next" node, use the previous bearing
	bearings.push(bearings[bearings.length - 1]);

	return bearings;
}

/* * */
