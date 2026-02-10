package services

import (
	"context"
	"fmt"
	"main/src/lib"
	"main/src/types"
	"strconv"
	"strings"

	_ "embed"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// *************
// *  Queries  *
// *************

//go:embed queries/fetch-vehicle-events.sql
var fetchVehicleEventsQuery string

//go:embed queries/unique-hashed-shapes-by-line.sql
var uniqueHashedShapesByLineQuery string

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
func (c *ClickhouseClient) FetchVehicleEvents(ctx context.Context, geohashes []string, settings *types.Settings) []types.VehicleEvent {
	if len(geohashes) == 0 {
		return nil
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

	query := strings.ReplaceAll(fetchVehicleEventsQuery, "{geohashes}", strings.Join(quotedGeohashes, ","))
	query = strings.ReplaceAll(query, "{min_events}", strconv.Itoa(settings.MinEvents))
	
	lib.AppLogger.Debug("Fetching vehicle events with query: %s", query)
	events, err := QueryAll(c, ctx, query, scanner);
	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to fetch vehicle events"))
	}

	return events
}

/**
	Fetches unique hashed shapes from the database.
	@return []string: The unique hashed shapes.
	@return error: The error if the request fails.
*/
func (c *ClickhouseClient) FetchUniqueHashedShapesIDsByLine(ctx context.Context) types.HashedShapeIdsByLineArray {

	// Scanner function to scan the unique hashed shapes
	scanner := func(rows driver.Rows) (types.HashedShapeIdsByLine, error) {
		var hashedShapeIdsByLine types.HashedShapeIdsByLine
		if err := rows.ScanStruct(&hashedShapeIdsByLine); err != nil {
			return types.HashedShapeIdsByLine{}, err
		}
		return hashedShapeIdsByLine, nil
	}
	
	hashedShapeIdsByLine, err := QueryAll(c, ctx, uniqueHashedShapesByLineQuery, scanner);

	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to fetch unique hashed shapes"))
	}

	
	return hashedShapeIdsByLine
}