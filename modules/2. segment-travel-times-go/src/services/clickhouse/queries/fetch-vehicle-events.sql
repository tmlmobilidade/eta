-- Filter vehicle events for specific geohash areas and deduplicate
WITH trip_events AS (
    SELECT *
    FROM vehicle_events 
    WHERE geohash IN ({geohashes})            -- Filter by geographic area
    ORDER BY ride_id, created_at
    LIMIT 1 BY                                -- Deduplicate events with same:
        ride_id,                              --   - Ride identifier
        created_at,                           --   - Timestamp
        latitude,                             --   - GPS coordinates
        longitude
)

-- Return only trips with sufficient data points (%d+ events)
SELECT *
FROM trip_events
WHERE ride_id IN (
    SELECT ride_id
    FROM trip_events
    GROUP BY ride_id
    HAVING count() >= {min_events}            -- Minimum events threshold
)
ORDER BY ride_id, created_at                  -- Chronological order per trip
