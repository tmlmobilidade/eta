package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"strings"
)

// CreateTravelTimesTable creates the segment_travel_times table in ClickHouse if it doesn't exist.
func (s *ClickhouseService) CreateTravelTimesTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS segment_travel_times (
			line_id UInt32,
			hashed_shape_id String,
			node_index UInt16,
			latitude Float64,
			longitude Float64,
			hour UInt8,
			travel_time_seconds Float32,
			sample_count UInt32
		) ENGINE = MergeTree()
		ORDER BY (line_id, hashed_shape_id, node_index, hour)
	`

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to create segment_travel_times table")
	}

	lib.AppLogger.Info("Created/verified segment_travel_times table")
	return nil
}

// SaveTravelTimes saves travel time records to ClickHouse using batch insert for efficiency.
func (s *ClickhouseService) SaveTravelTimes(ctx context.Context, records []types.NodeTravelTimeRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO segment_travel_times")
	if err != nil {
		return lib.AppLogger.Error(err, "failed to prepare batch")
	}

	for _, record := range records {
		if err := batch.Append(
			record.LineID,
			record.HashedShapeID,
			record.NodeIndex,
			record.Latitude,
			record.Longitude,
			record.Hour,
			record.TravelTimeSeconds,
			record.SampleCount,
		); err != nil {
			return lib.AppLogger.Error(err, "failed to append record to batch")
		}
	}

	if err := batch.Send(); err != nil {
		return lib.AppLogger.Error(err, "failed to send batch")
	}

	return nil
}

// DeleteTravelTimesForShapes deletes existing travel time records for specific line_id and hashed_shape_ids.
// Used to prevent duplicate data when reprocessing.
func (s *ClickhouseService) DeleteTravelTimesForShapes(ctx context.Context, lineID uint32, hashedShapeIDs []string) error {
	if len(hashedShapeIDs) == 0 {
		return nil
	}

	// Build the IN clause with quoted strings
	quotedIDs := make([]string, len(hashedShapeIDs))
	for i, id := range hashedShapeIDs {
		quotedIDs[i] = fmt.Sprintf("'%s'", id)
	}

	query := fmt.Sprintf(`
		ALTER TABLE segment_travel_times 
		DELETE WHERE line_id = %d 
		AND hashed_shape_id IN (%s)
	`, lineID, strings.Join(quotedIDs, ","))

	if err := s.conn.Exec(ctx, query); err != nil {
		return lib.AppLogger.Error(err, "failed to delete travel times for shapes")
	}

	lib.AppLogger.Debug("Deleted existing travel time records for line %d with %d shapes", lineID, len(hashedShapeIDs))
	return nil
}

// FetchVehicleEvents fetches vehicle events from ClickHouse for the given geohashes.
// Events are filtered by date range and grouped by trip_operational_id.
func (s *ClickhouseService) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error) {
	if len(geohashes) == 0 {
		return nil, nil
	}

	// Build the geohash column name based on precision
	geohashColumn := fmt.Sprintf("geohash_%d", settings.GeohashPrecision)

	// Build the IN clause with quoted strings
	quotedGeohashes := make([]string, len(geohashes))
	for i, gh := range geohashes {
		quotedGeohashes[i] = fmt.Sprintf("'%s'", gh)
	}

	query := fmt.Sprintf(`
		SELECT
			Concat(trip_id, '-', toString(operational_date)) AS trip_operational_id,
			%s AS geohash,
			created_at,
			latitude,
			longitude
		FROM vehicle_events 
		WHERE created_at >= %d AND created_at < %d
		AND Char_length(trip_id) > 0
		AND %s IN (%s)
		ORDER BY trip_operational_id, created_at
		LIMIT 1 BY
			concat(trip_id, '-', toString(operational_date)),
			geohash_7,
			created_at,
			latitude,
			longitude
	`, geohashColumn, settings.RideStartDate, settings.RideEndDate, geohashColumn, strings.Join(quotedGeohashes, ","))

	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return nil, lib.AppLogger.Error(err, "failed to fetch vehicle events")
	}
	defer rows.Close()

	var events []types.VehicleEvent
	for rows.Next() {
		var event types.VehicleEvent
		if err := rows.Scan(
			&event.TripOperationalID,
			&event.Geohash,
			&event.CreatedAt,
			&event.Latitude,
			&event.Longitude,
		); err != nil {
			return nil, lib.AppLogger.Error(err, "failed to scan vehicle event")
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, lib.AppLogger.Error(err, "error iterating vehicle events")
	}

	return events, nil
}
