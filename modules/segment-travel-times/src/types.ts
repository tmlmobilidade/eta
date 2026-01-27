/* * */

import type { OperationalDate, UnixTimestamp } from '@tmlmobilidade/types';

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
	operational_date: OperationalDate
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
	/** Bearing threshold in degrees for filtering events (0-180) */
	bearingThreshold: number
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

/**
 * Event matched to a specific node index
 */
export interface NodeEventMatch {
	/** Unix timestamp when the event occurred */
	created_at: number
	/** Hour of the day (0-23) extracted from created_at */
	hour: number
	/** Index of the matched node in the shape */
	nodeIndex: number
	/** Operational date of the event */
	operational_date: OperationalDate
}

/**
 * Travel time sample for a node
 */
export interface NodeTravelTimeSample {
	/** Hour of the day (0-23) */
	hour: number
	/** Index of the node in the shape */
	nodeIndex: number
	/** Travel time in seconds to reach this node from the previous matched node */
	travelTimeSeconds: number
}

/**
 * Aggregated travel time data for a node by hour
 */
export interface HourlyNodeTravelTime {
	/** Hour of the day (0-23) */
	hour: number
	/** Number of samples used for this aggregation */
	sampleCount: number
	/** Average travel time in seconds */
	travelTimeSeconds: number
}

/**
 * Map of hour (0-23) to aggregated travel time data
 */
export type HourlyTravelTimes = Map<number, { sampleCount: number, totalTravelTime: number }>;

/**
 * Record structure for ClickHouse storage
 */
export interface NodeTravelTimeRecord {
	/** Hashed shape ID */
	hashed_shape_id: string
	/** Hour of the day (0-23) */
	hour: number
	/** Latitude of the node */
	latitude: number
	/** Line ID */
	line_id: number
	/** Longitude of the node */
	longitude: number
	/** Index of the node in the shape */
	node_index: number
	/** Number of samples */
	sample_count: number
	/** Average travel time in seconds */
	travel_time_seconds: number
}

/* * */
