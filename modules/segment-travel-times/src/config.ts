/* * */

import type { SegmentTravelTimesSettings } from './types.js';

import { Dates } from '@tmlmobilidade/dates';

/* * */

/** Run interval in milliseconds (10 minutes) */
export const RUN_INTERVAL = 60_000 * 10;

/**
 * Creates default settings for segment travel times calculation
 *
 * @returns Settings with current date range (last 7 days)
 */
export function createDefaultSettings(): SegmentTravelTimesSettings {
	return {
		geohashPrecision: 7,
		rideEndDate: Dates.now('Europe/Lisbon').set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
		rideStartDate: Dates.now('Europe/Lisbon').minus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
		segmentLengthMeters: 50,
	};
}

/* * */
