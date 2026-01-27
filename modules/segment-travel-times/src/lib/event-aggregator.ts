/* * */

import type { TripEventCounts } from '../types.js';

/* * */

/**
 * Aggregates vehicle events from cache for a set of geohashes
 * Groups events by trip_id and counts occurrences
 *
 * @param geohashes - Array of geohashes to aggregate
 * @param cache - The geohash events cache
 * @returns Map of trip_id to event count
 */
export function aggregateEventsByGeohash(geohashes: string[]): TripEventCounts {
	const groupedEvents: TripEventCounts = new Map();

	for (const gh of geohashes) {
		const currentCount = groupedEvents.get(gh) ?? 0;
		groupedEvents.set(gh, currentCount + 1);
	}

	return groupedEvents;
}

/**
 * Converts TripEventCounts to a sorted array for display
 *
 * @param counts - Map of trip_id to count
 * @param sortBy - Sort by 'count' (descending) or 'trip_id' (ascending)
 * @returns Sorted array of [trip_id, count] tuples
 */
export function sortedTripEventCounts(
	counts: TripEventCounts,
	sortBy: 'count' | 'trip_id' = 'count',
): [string, number][] {
	const entries = Array.from(counts.entries());

	if (sortBy === 'count') {
		return entries.sort(([, a], [, b]) => b - a);
	}
	return entries.sort(([a], [b]) => a.localeCompare(b));
}

/* * */
