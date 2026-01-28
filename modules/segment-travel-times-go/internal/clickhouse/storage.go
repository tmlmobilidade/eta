package clickhouse

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

// CreateTravelTimesTable creates the segment_travel_times table if it doesn't exist.
func (c *Client) CreateTravelTimesTable(ctx context.Context) error {
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

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create segment_travel_times table: %w", err)
	}
	log.Println("Created/verified segment_travel_times table")
	return nil
}

// SaveTravelTimes saves travel time records to ClickHouse using batch insert.
func (c *Client) SaveTravelTimes(ctx context.Context, records []types.NodeTravelTimeRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO segment_travel_times")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, r := range records {
		err := batch.Append(
			uint32(r.LineID),
			r.HashedShapeID,
			uint16(r.NodeIndex),
			r.Latitude,
			r.Longitude,
			uint8(r.Hour),
			float32(r.TravelTimeSeconds),
			uint32(r.SampleCount),
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

// DeleteTravelTimesForShapes deletes existing travel time records for specific line_id and hashed_shape_ids.
// Used to prevent duplicate data when reprocessing.
func (c *Client) DeleteTravelTimesForShapes(ctx context.Context, lineID int, hashedShapeIDs []string) error {
	if len(hashedShapeIDs) == 0 {
		return nil
	}

	// Build quoted list of shape IDs
	quotedIDs := make([]string, len(hashedShapeIDs))
	for i, id := range hashedShapeIDs {
		quotedIDs[i] = fmt.Sprintf("'%s'", id)
	}

	query := fmt.Sprintf(`
		ALTER TABLE segment_travel_times 
		DELETE WHERE line_id = %d 
		AND hashed_shape_id IN (%s)
	`, lineID, strings.Join(quotedIDs, ","))

	if err := c.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to delete travel times for line %d: %w", lineID, err)
	}

	log.Printf("Deleted existing travel time records for line %d with %d shapes", lineID, len(hashedShapeIDs))
	return nil
}
