/* * */

import type { UnixTimestamp } from '@tmlmobilidade/types';

/**
 * A coordinate pair [longitude, latitude] in GeoJSON order
 */
export type Coordinate = [number, number];

/**
 * Shape data associated with a single line
 * Contains all hashed shapes, their segment nodes, and unique geohashes
 */
export interface LineShapeData {
	/** Set of unique geohashes covering all segment endpoints */
	geohashes: Set<string>
	/** Array of hashed shape IDs belonging to this line */
	hashedShapeIds: string[]
	/** Map from hashed_shape_id to its segment endpoint coordinates */
	nodes: Map<string, Coordinate[]>
}

/**
 * A Map structure grouping LineShapeData by line_id
 */
export type LineShapesMap = Map<number, LineShapeData>;

/**
 * A cached vehicle event with geohash association
 */
export interface VehicleEvent {
	created_at: number
	geohash: string
	latitude: number
	longitude: number
	operational_date: number
	trip_operational_id: string
}

/**
 * Aggregated event counts by trip_id
 */
export type TripEventCounts = Map<string, number>;

/**
 * Settings for the segment travel times calculation
 */
export interface SegmentTravelTimesSettings {
	/** Geohash precision level (typically 6-8) */
	geohashPrecision: number
	/** Unix timestamp for ride query end date */
	rideEndDate: UnixTimestamp
	/** Unix timestamp for ride query start date */
	rideStartDate: UnixTimestamp
	/** Length of each segment in meters */
	segmentLengthMeters: number
}

/**
 * Ride document projection for the aggregation query
 */
export interface RideProjection {
	_id: string
	hashed_shape_id: string
	line_id: number
	start_time_scheduled: number
	trip_id: string
	vehicle_ids: string[]
}

/**
 * Point data from a hashed shape
 */
export interface ShapePoint {
	shape_pt_lat: number
	shape_pt_lon: number
}

/**
 * Hashed shape with projected point fields
 */
export interface HashedShapePointProjection {
	_id: string
	points: ShapePoint[]
}

/* * */
