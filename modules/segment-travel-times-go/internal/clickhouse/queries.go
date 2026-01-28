package clickhouse

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

// FetchVehicleEvents fetches vehicle events from ClickHouse for the given geohashes.
func (c *Client) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error) {
	if len(geohashes) == 0 {
		return []types.VehicleEvent{}, nil
	}

	geohashColumn := fmt.Sprintf("geohash_%d", settings.GeohashPrecision)

	// Build quoted list of geohashes
	quotedHashes := make([]string, len(geohashes))
	for i, hash := range geohashes {
		quotedHashes[i] = fmt.Sprintf("'%s'", hash)
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
	`, geohashColumn, settings.RideStartDate, settings.RideEndDate, geohashColumn, strings.Join(quotedHashes, ","))

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query vehicle events: %w", err)
	}
	defer rows.Close()

	var events []types.VehicleEvent
	rowCount := 0

	for rows.Next() {
		var event types.VehicleEvent
		if err := rows.Scan(
			&event.TripOperationalID,
			&event.Geohash,
			&event.CreatedAt,
			&event.Latitude,
			&event.Longitude,
		); err != nil {
			return nil, fmt.Errorf("failed to scan vehicle event: %w", err)
		}
		events = append(events, event)
		rowCount++

		if rowCount%500000 == 0 {
			log.Printf("Fetched %d rows...", rowCount)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vehicle events: %w", err)
	}

	log.Printf("Fetched %d events for %d geohashes", len(events), len(geohashes))
	return events, nil
}
