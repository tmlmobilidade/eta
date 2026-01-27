/* * */

import type { SegmentTravelTimesSettings, VehicleEvent } from '../types.js';

import { ClickHouseClient } from '@clickhouse/client';
import { Logger } from '@tmlmobilidade/logger';

/* * */

/**
 * Builds a ClickHouse query for fetching vehicle events by geohashes
 *
 * @param geohashes - Array of geohashes to query
 * @param settings - Settings containing date range and geohash precision
 * @returns Complete SQL query string
 */
function buildVehicleEventsQuery(geohashes: string[], settings: SegmentTravelTimesSettings): string {
	const geohashColumn = `geohash_${settings.geohashPrecision}`;
	const whereClause = `
		WHERE created_at >= ${settings.rideStartDate} AND created_at < ${settings.rideEndDate}
		AND Char_length(trip_id) > 0
		AND ${geohashColumn} IN ('${geohashes.join('\',\'')}')
	`;
	return `
		SELECT
			Concat(trip_id, '-', toString(operational_date)) AS trip_operational_id,
			${geohashColumn} AS geohash,
			created_at,
			latitude,
			longitude
		FROM vehicle_events 
		${whereClause} 
		ORDER BY trip_operational_id, created_at
		LIMIT 1 BY
			concat(trip_id, '-', toString(operational_date)),
			geohash_7,
			created_at,
			latitude,
			longitude;`;
}

/**
 * Fetches vehicle events from ClickHouse and populates the cache
 *
 * @param client - ClickHouse client
 * @param geohashes - Array of geohashes to fetch
 * @param cache - Cache to populate
 * @param settings - Settings for the query
 * @returns Number of events fetched
 */
export async function fetchVehicleEvents(client: ClickHouseClient, geohashes: string[], settings: SegmentTravelTimesSettings): Promise<VehicleEvent[]> {
	if (geohashes.length === 0) {
		return [];
	}

	// Execute query
	const query = buildVehicleEventsQuery(geohashes, settings);
	const result = await client.query({
		format: 'JSONEachRow',
		query,
	});

	// Stream results into cache
	const vehicleEvents: VehicleEvent[] = [];
	let rowCount = 0;
	for await (const rows of result.stream()) {
		for (const row of rows) {
			rowCount++;
			if (rowCount % 500_000 === 0) {
				Logger.info(`Fetched ${rowCount} rows...`);
			}

			vehicleEvents.push(row.json<VehicleEvent>());
		}
	}

	Logger.info(`Fetched ${vehicleEvents.length} events for ${geohashes.length} geohashes`);
	return vehicleEvents;
}
/* * */
