/* * */

import { ClickHouseClient, createClient } from '@clickhouse/client';

/* * */

/**
 * Creates a configured ClickHouse client using environment variables
 *
 * @returns Configured ClickHouse client instance
 */
export function createClickHouseClient(): ClickHouseClient {
	return createClient({
		database: process.env.CLICKHOUSE_DATABASE,
		password: process.env.CLICKHOUSE_PASSWORD,
		url: `http://${process.env.CLICKHOUSE_HOST}:${process.env.CLICKHOUSE_PORT}`,
		username: process.env.CLICKHOUSE_USERNAME,
	});
}

/* * */
