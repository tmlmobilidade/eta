/* * */

import { Dates } from '@tmlmobilidade/dates';
import { SimplifiedVehicleEvent } from '@tmlmobilidade/types';
import geohash from 'ngeohash';

import { EtaVehicleEvent } from './types.js';

/* * */

/**
 * Transform a SimplifiedVehicleEvent document for ClickHouse insertion.
 * Flattens the nested position object and handles type conversions.
 */
// // eslint-disable-next-line @typescript-eslint/no-explicit-any
// export function transformVehicleEventForClickHouse(pcgiDoc: any, hashedShapeId: string): EtaVehicleEvent {
// 	const entity = pcgiDoc.content.entity[0];
// 	const operationalDate = Dates.fromSeconds(entity.vehicle.timestamp).operational_date;

// 	return {
// 		_id: pcgiDoc._id,
// 		agency_id: entity.vehicle.agencyId,
// 		created_at: Dates.fromSeconds(entity.vehicle.timestamp).unix_timestamp,
// 		geohash: geohash.encode(entity.vehicle.position.latitude, entity.vehicle.position.longitude, 7),
// 		hashed_shape_id: hashedShapeId,
// 		latitude: entity.vehicle.position.latitude,
// 		longitude: entity.vehicle.position.longitude,
// 		operational_date: operationalDate,
// 		trip_id: entity.vehicle.trip?.tripId,
// 		vehicle_id: entity.vehicle.vehicle._id,
// 	};
// }

export function parseToEtaVehicleEvent(simplifiedVehicleEvent: SimplifiedVehicleEvent, hashedShapeId: string): EtaVehicleEvent {
	const operationalDate = Dates.fromSeconds(simplifiedVehicleEvent.created_at).operational_date;

	return {
		_id: simplifiedVehicleEvent._id,
		agency_id: simplifiedVehicleEvent.agency_id,
		created_at: simplifiedVehicleEvent.created_at,
		geohash: geohash.encode(simplifiedVehicleEvent.latitude, simplifiedVehicleEvent.longitude, 7),
		hashed_shape_id: hashedShapeId,
		latitude: simplifiedVehicleEvent.latitude,
		longitude: simplifiedVehicleEvent.longitude,
		operational_date: operationalDate,
		trip_id: simplifiedVehicleEvent.trip_id,
		vehicle_id: simplifiedVehicleEvent.vehicle_id,
	};
}
