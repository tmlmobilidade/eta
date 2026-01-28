/* * */

import type { RideProjection, SegmentTravelTimesSettings } from '../types.js';

import { AggregationPipeline, rides } from '@tmlmobilidade/interfaces';

/* * */

/**
 * Builds the aggregation pipeline for fetching rides within the date range
 *
 * @param settings - Settings containing date range
 * @returns MongoDB aggregation pipeline
 */
function buildRidesAggregationPipeline(settings: SegmentTravelTimesSettings): AggregationPipeline<RideProjection> {
	return [
		{ $match: { start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate } } },
		{ $match: { agency_id: { $in: ['41', '42', '43', '44'] } } },
		{ $match: { line_id: { $gte: 1001, $lte: 1500 } } }, // ! DEBUG
		{
			$project: {
				_id: 0,
				hashed_shape_id: 1,
				line_id: 1,
				start_time_scheduled: 1,
				trip_id: 1,
				vehicle_ids: 1,
			},
		},
	];
}

/**
 * Fetches rides cursor and total count for the given settings
 *
 * @param settings - Settings containing date range
 * @returns Object containing cursor and total count
 */
export async function fetchRidesCursor(settings: SegmentTravelTimesSettings): Promise<{ cursor: AsyncIterable<RideProjection>, totalCount: number }> {
	const collection = await rides.getCollection();
	const pipeline = buildRidesAggregationPipeline(settings);

	const totalCount = await collection.countDocuments({ start_time_scheduled: { $gte: settings.rideStartDate, $lt: settings.rideEndDate } });
	const cursor = collection.aggregate<RideProjection>(pipeline).batchSize(100_000);

	return { cursor, totalCount };
}

/* * */
