/* * */

import type { Coordinate } from '../types.js';

import geohash from 'ngeohash';

/* * */

/**
 * Encodes a coordinate to a geohash string
 *
 * @param coordinate - [longitude, latitude] coordinate pair
 * @param precision - Geohash precision level
 * @returns Geohash string
 */
export function encodeCoordinate(coordinate: Coordinate, precision: number): string {
	// Note: geohash.encode expects (lat, lon), but Coordinate is [lon, lat]
	return geohash.encode(coordinate[1], coordinate[0], precision);
}

/**
 * Encodes multiple coordinates to geohash strings
 *
 * @param coordinates - Array of [longitude, latitude] coordinates
 * @param precision - Geohash precision level
 * @returns Array of geohash strings
 */
export function encodeCoordinates(coordinates: Coordinate[], precision: number): string[] {
	return coordinates.map(coord => encodeCoordinate(coord, precision));
}

/**
 * Encodes coordinates and returns unique geohashes as a Set
 *
 * @param coordinates - Array of [longitude, latitude] coordinates
 * @param precision - Geohash precision level
 * @returns Set of unique geohash strings
 */
export function encodeCoordinatesToSet(coordinates: Coordinate[], precision: number): Set<string> {
	return new Set(encodeCoordinates(coordinates, precision));
}

/* * */
