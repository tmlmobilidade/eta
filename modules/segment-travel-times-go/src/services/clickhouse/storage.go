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
		return lib.AppLogger.Error("failed to create segment_travel_times table", err.Error())
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
		return lib.AppLogger.Error("failed to prepare batch", err.Error())
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
			return lib.AppLogger.Error("failed to append record to batch", err.Error())
		}
	}

	if err := batch.Send(); err != nil {
		return lib.AppLogger.Error("failed to send batch", err.Error())
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
		return lib.AppLogger.Error("failed to delete travel times for shapes", err.Error())
	}

	lib.AppLogger.Info(fmt.Sprintf("Deleted existing travel time records for line %d with %d shapes", lineID, len(hashedShapeIDs)))
	return nil
}
