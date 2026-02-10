package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"strings"

	_ "embed"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// *************
// *  Queries  *
// *************

//go:embed queries/fetch-vehicle-events.sql
var fetchVehicleEventsQuery string

//go:embed queries/unique-hashed-shapes.sql
var uniqueHashedShapesQuery string

// *************
// * Functions *
// *************

/**
	Fetches Vehicle Events from filtered by geolocation.
	Returns deduplicated vehicle events by ride_id and created_at.
	Only returns trips with sufficient data points (min_events).

	@param ctx context.Context: The context for the request.
	@param geohashes []string: The geohashes to filter the vehicle events by.
	@param settings *types.Settings: The settings for the request (min_events).
	@return []types.VehicleEvent: The vehicle events.
	@return error: The error if the request fails.
*/
func (c *ClickhouseClient) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) ([]types.VehicleEvent, error) {
	if len(geohashes) == 0 {
		return nil, nil
	}

	// Build the IN clause with quoted strings
	quotedGeohashes := make([]string, len(geohashes))
	for i, gh := range geohashes {
		quotedGeohashes[i] = fmt.Sprintf("'%s'", gh)
	}

	// Scanner function to scan the vehicle events
	scanner := func(rows driver.Rows) (types.VehicleEvent, error) {
		var event types.VehicleEvent
		if err := rows.ScanStruct(&event); err != nil {
			return types.VehicleEvent{}, err
		}
		return event, nil
	}

	query := fmt.Sprintf(fetchVehicleEventsQuery, strings.Join(quotedGeohashes, ","), settings.MinEvents)
	events, err := QueryAll(c, ctx, query, scanner);
	if err != nil {
		return nil, lib.AppLogger.Error(err, "failed to fetch vehicle events")
	}

	return events, nil
}

/**
	Fetches unique hashed shapes from the database.
	@return []string: The unique hashed shapes.
	@return error: The error if the request fails.
*/
func (c *ClickhouseClient) FetchUniqueHashedShapes(ctx context.Context) ([]string, error) {

	// Scanner function to scan the unique hashed shapes
	scanner := func(rows driver.Rows) (string, error) {
		var shape string
		if err := rows.Scan(&shape); err != nil {
			return "", err
		}
		return shape, nil
	}
	
	shapes, err := QueryAll(c, ctx, uniqueHashedShapesQuery, scanner);

	if err != nil {
		return nil, lib.AppLogger.Error(err, "failed to fetch unique hashed shapes")
	}
	return shapes, nil
}