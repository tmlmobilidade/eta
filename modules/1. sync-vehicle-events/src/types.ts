import { OperationalDate, UnixTimestamp } from '@tmlmobilidade/types';
import { ClickHouseColumn } from '@tmlmobilidade/writers';

export interface EtaVehicleEvent {
	_id: string
	agency_id: string
	created_at: number
	geohash: string
	hashed_shape_id: string
	latitude: number
	longitude: number
	operational_date: OperationalDate
	trip_id: string
	vehicle_id: string
}

export type RidesMap = Map<string, { hashed_shape_id: string, start_time_observed: UnixTimestamp }>;

export const EtaVehicleEventTableSchema: ClickHouseColumn<EtaVehicleEvent>[] = [
	{ name: '_id', type: 'String' },
	{ name: 'agency_id', type: 'String' },
	{ name: 'created_at', type: 'UInt64' },
	{ name: 'geohash', type: 'String' },
	{ name: 'hashed_shape_id', type: 'String' },
	{ name: 'latitude', type: 'Float64' },
	{ name: 'longitude', type: 'Float64' },
	{ name: 'operational_date', type: 'String' },
	{ name: 'trip_id', type: 'String' },
	{ name: 'vehicle_id', type: 'String' },
];
