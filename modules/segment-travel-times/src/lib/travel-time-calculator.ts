/* * */

import type { Coordinate, NodeEventMatch, NodeTravelTimeRecord, NodeTravelTimeSample, VehicleEvent } from '../types.js';

import * as turf from '@turf/turf';

import { calculateBearing, calculateShapeBearings, isValidBearing } from '../utils/bearing-utils.js';

/* * */

/**
 * Groups vehicle events by their trip_operational_id
 * Events within each group are already sorted by created_at (from ClickHouse query)
 *
 * @param events - Array of vehicle events
 * @returns Map from trip_operational_id to array of events
 */
export function groupEventsByTrip(events: VehicleEvent[]): Map<string, VehicleEvent[]> {
	const grouped = new Map<string, VehicleEvent[]>();

	for (const event of events) {
		const existing = grouped.get(event.trip_operational_id);
		if (existing) {
			existing.push(event);
		}
		else {
			grouped.set(event.trip_operational_id, [event]);
		}
	}

	return grouped;
}

/**
 * Finds the nearest node index for a given event coordinate
 *
 * @param eventLon - Event longitude
 * @param eventLat - Event latitude
 * @param nodes - Array of node coordinates
 * @returns Index of the nearest node
 */
function findNearestNodeIndex(eventLon: number, eventLat: number, nodes: Coordinate[]): number {
	const eventPoint = turf.point([eventLon, eventLat]);
	let nearestIndex = 0;
	let minDistance = Infinity;

	for (let i = 0; i < nodes.length; i++) {
		const nodePoint = turf.point(nodes[i]);
		const distance = turf.distance(eventPoint, nodePoint, { units: 'meters' });
		if (distance < minDistance) {
			minDistance = distance;
			nearestIndex = i;
		}
	}

	return nearestIndex;
}

/**
 * Extracts the hour (0-23) from a Unix timestamp
 * Uses UTC for consistency
 * Note: created_at is in milliseconds, so we convert to seconds first
 *
 * @param unixTimestampMs - Unix timestamp in milliseconds
 * @returns Hour of the day (0-23)
 */
function extractHour(unixTimestampMs: number): number {
	return new Date(unixTimestampMs).getUTCHours();
}

/**
 * Filters and matches vehicle events to shape nodes based on bearing
 * Only events traveling in the same direction as the shape are included
 *
 * @param tripEvents - Events for a single trip, sorted by created_at
 * @param nodes - Shape node coordinates
 * @param shapeBearings - Precomputed bearings for each node
 * @param bearingThreshold - Maximum bearing difference in degrees
 * @returns Array of matched events with their node indices
 */
export function matchEventsToNodes(
	tripEvents: VehicleEvent[],
	nodes: Coordinate[],
	shapeBearings: number[],
	bearingThreshold: number,
): NodeEventMatch[] {
	if (tripEvents.length < 2 || nodes.length < 2) {
		return [];
	}

	const matches: NodeEventMatch[] = [];

	// Process consecutive event pairs to check bearing
	for (let i = 0; i < tripEvents.length - 1; i++) {
		const currentEvent = tripEvents[i];
		const nextEvent = tripEvents[i + 1];

		// Calculate event bearing (direction of travel)
		const eventBearing = calculateBearing(
			[currentEvent.longitude, currentEvent.latitude],
			[nextEvent.longitude, nextEvent.latitude],
		);

		// Find nearest node to the current event
		const nodeIndex = findNearestNodeIndex(currentEvent.longitude, currentEvent.latitude, nodes);

		// Check if event bearing matches shape bearing at this node
		const shapeBearing = shapeBearings[nodeIndex];
		if (!isValidBearing(eventBearing, shapeBearing, bearingThreshold)) {
			// Event is traveling in wrong direction, skip
			continue;
		}

		matches.push({
			created_at: currentEvent.created_at,
			hour: extractHour(currentEvent.created_at),
			nodeIndex,
			operational_date: currentEvent.operational_date,
		});
	}

	// Handle the last event if we have previous valid matches
	if (matches.length > 0) {
		const lastEvent = tripEvents[tripEvents.length - 1];
		const lastNodeIndex = findNearestNodeIndex(lastEvent.longitude, lastEvent.latitude, nodes);

		// Only add if it advances along the shape (prevents duplicates)
		const lastMatch = matches[matches.length - 1];
		if (lastNodeIndex > lastMatch.nodeIndex) {
			matches.push({
				created_at: lastEvent.created_at,
				hour: extractHour(lastEvent.created_at),
				nodeIndex: lastNodeIndex,
				operational_date: lastEvent.operational_date,
			});
		}
	}

	return matches;
}

/**
 * Calculates travel times from matched events and distributes them across nodes
 * If events match nodes A and D with B and C in between, the travel time is
 * distributed evenly across all nodes from A to D
 *
 * @param matches - Array of events matched to nodes (sorted by time)
 * @returns Array of travel time samples for each node
 */
export function calculateTravelTimeSamples(matches: NodeEventMatch[]): NodeTravelTimeSample[] {
	if (matches.length < 2) {
		return [];
	}

	const samples: NodeTravelTimeSample[] = [];

	for (let i = 0; i < matches.length - 1; i++) {
		const startMatch = matches[i];
		const endMatch = matches[i + 1];

		// Skip if nodes are not advancing (vehicle might have stopped or gone backwards)
		if (endMatch.nodeIndex <= startMatch.nodeIndex) {
			continue;
		}

		// Convert from milliseconds to seconds for time difference calculation
		const timeDiffMs = endMatch.created_at - startMatch.created_at;
		const timeDiffSeconds = timeDiffMs / 1000;
		const nodeCount = endMatch.nodeIndex - startMatch.nodeIndex;

		// Distribute time evenly across all nodes in the segment
		const timePerNode = timeDiffSeconds / nodeCount;

		// Assign travel time to each node in the segment (excluding the start node)
		for (let nodeIdx = startMatch.nodeIndex + 1; nodeIdx <= endMatch.nodeIndex; nodeIdx++) {
			samples.push({
				hour: startMatch.hour,
				nodeIndex: nodeIdx,
				travelTimeSeconds: timePerNode,
			});
		}
	}

	return samples;
}

/**
 * Aggregates travel time samples by node index and hour
 *
 * @param samples - Array of individual travel time samples
 * @returns Map keyed by "nodeIndex-hour" with aggregated data
 */
export function aggregateSamples(
	samples: NodeTravelTimeSample[],
): Map<string, { hour: number, nodeIndex: number, sampleCount: number, totalTravelTime: number }> {
	const aggregated = new Map<string, { hour: number, nodeIndex: number, sampleCount: number, totalTravelTime: number }>();

	for (const sample of samples) {
		const key = `${sample.nodeIndex}-${sample.hour}`;
		const existing = aggregated.get(key);

		if (existing) {
			existing.sampleCount++;
			existing.totalTravelTime += sample.travelTimeSeconds;
		}
		else {
			aggregated.set(key, {
				hour: sample.hour,
				nodeIndex: sample.nodeIndex,
				sampleCount: 1,
				totalTravelTime: sample.travelTimeSeconds,
			});
		}
	}

	return aggregated;
}

/**
 * Processes all vehicle events for a shape and calculates travel times
 * This is the main entry point for travel time calculation
 *
 * @param events - All vehicle events for the geohashes covering this shape
 * @param nodes - Shape node coordinates
 * @param lineId - Line ID for the shape
 * @param hashedShapeId - Hashed shape ID
 * @param bearingThreshold - Maximum bearing difference in degrees
 * @returns Array of records ready for ClickHouse insertion
 */
export function processShapeTravelTimes(
	events: VehicleEvent[],
	nodes: Coordinate[],
	lineId: number,
	hashedShapeId: string,
	bearingThreshold: number,
): NodeTravelTimeRecord[] {
	if (events.length === 0 || nodes.length < 2) {
		return [];
	}

	// Calculate bearings for all node pairs
	const shapeBearings = calculateShapeBearings(nodes);

	// Group events by trip
	const tripGroups = groupEventsByTrip(events);

	// Collect all samples from all trips
	const allSamples: NodeTravelTimeSample[] = [];

	for (const [, tripEvents] of tripGroups) {
		// Skip trips with only one event
		if (tripEvents.length < 2) {
			continue;
		}

		// Match events to nodes, filtering by bearing
		const matches = matchEventsToNodes(tripEvents, nodes, shapeBearings, bearingThreshold);

		// Calculate travel times from matches
		const samples = calculateTravelTimeSamples(matches);
		allSamples.push(...samples);
	}

	// Aggregate samples by node and hour
	const aggregated = aggregateSamples(allSamples);

	// Convert to records for ClickHouse
	const records: NodeTravelTimeRecord[] = [];
	for (const data of aggregated.values()) {
		const node = nodes[data.nodeIndex];
		records.push({
			hashed_shape_id: hashedShapeId,
			hour: data.hour,
			latitude: node[1],
			line_id: lineId,
			longitude: node[0],
			node_index: data.nodeIndex,
			sample_count: data.sampleCount,
			travel_time_seconds: data.totalTravelTime / data.sampleCount,
		});
	}

	return records;
}

/* * */
