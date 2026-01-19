/* * */

import { Dates } from '@tmlmobilidade/dates';
import { AggregationPipeline, rides } from '@tmlmobilidade/interfaces';
import { Logger } from '@tmlmobilidade/logger';
import { Timer } from '@tmlmobilidade/timer';
import { Ride } from '@tmlmobilidade/types';

/* * */

const RUN_INTERVAL = 60_000 * 60 * 24; // 1 day in milliseconds

const settings = {
	rideEndDate: Dates.now('Europe/Lisbon').plus({ days: 1 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
	rideStartDate: Dates.now('Europe/Lisbon').minus({ days: 7 }).set({ hour: 4, minute: 0, second: 0 }).unix_timestamp,
};

async function main() {
	//

	Logger.init();

	const globalTimer = new Timer();

	//
	// 1. Get All Unique Hashed Shape IDs
	const aggregationPipeline: AggregationPipeline<Ride> = [
		{
			$match: {
				start_time_scheduled: {
					$gte: settings.rideStartDate,
					$lt: settings.rideEndDate,
				},
			},
		},
		{
			$group: {
				_id: null,
				hashed_shape_ids: { $addToSet: '$hashed_shape_id' },
			},
		},
		{
			$project: {
				_id: 0,
				hashed_shape_ids: 1,
			},
		},
	];

	const result = (await rides.aggregate(aggregationPipeline)) as unknown as { hashed_shape_ids: string[] }[];
	const hashedShapeIds = result[0]?.hashed_shape_ids ?? [];

	console.log(hashedShapeIds);

	Logger.terminate(`Terminated in ${globalTimer.get()}`);

	//
}

/* * */

(async function init() {
	const runOnInterval = async () => {
		await main();
		setTimeout(runOnInterval, RUN_INTERVAL);
	};
	runOnInterval();
})();
