INSERT INTO node_travel_times_samples
WITH
    -- CONFIGURATION (Adjust constants here)
    25 AS segment_length_m,
    30 AS max_dist_m,
    90 AS bearing_threshold_deg,

    -- 1. SNAP: Find the nearest node for every GPS ping
    matched_events AS (
        SELECT 
            e.ride_id,
            e.hashed_shape_id,
            e.created_at,
            e.latitude AS e_lat,
            e.longitude AS e_lon,
            argMin(
                n.node_index,
                greatCircleDistance(e.longitude, e.latitude, n.lon, n.lat)
            ) AS node_idx,
            min(greatCircleDistance(e.longitude, e.latitude, n.lon, n.lat)) AS dist
        FROM vehicle_events AS e
        INNER JOIN shape_nodes AS n ON e.hashed_shape_id = n.shape_id
        WHERE e.created_at > 0 -- Filter junk
        GROUP BY e.ride_id, e.hashed_shape_id, e.created_at, e_lat, e_lon
        HAVING dist <= max_dist_m
    ),

    -- 2. PAIR & CALCULATE: Look at consecutive events in the same ride
    segments AS (
        SELECT 
            hashed_shape_id,
            ride_id,
            node_idx AS curr_idx, 
            lag(node_idx, 1) OVER w AS prev_idx,
            created_at AS curr_ts, 
            lag(created_at, 1) OVER w AS prev_ts,
            -- Spatial data for bearing
            e_lat AS curr_lat,
            e_lon AS curr_lon,
            lag(e_lat, 1) OVER w AS prev_lat,
            lag(e_lon, 1) OVER w AS prev_lon,
            -- Calculations
            (curr_ts - prev_ts) / 1000 AS time_delta,
            (curr_idx - prev_idx) * segment_length_m AS dist_m,
            (dist_m / time_delta) * 3.6 AS speed_kmh
        FROM matched_events
        WINDOW w AS (PARTITION BY ride_id ORDER BY created_at)
    ),

    -- 3. VALIDATE: Filter by direction, speed, and bearing
    filtered_segments AS (
        SELECT 
            *
        FROM 
            segments
        WHERE 
          prev_idx IS NOT NULL               -- Ensure there is a previous event in the ride
          AND curr_idx > prev_idx            -- Forward travel
          AND speed_kmh BETWEEN 1 AND 120    -- Speed filter
          AND (
              -- BEARING CHECK: atan2 gives direction in radians
              -- We ensure the event movement matches the shape direction
              abs(
                  atan2(curr_lat - prev_lat, curr_lon - prev_lon) - 
                  atan2(
                      (SELECT lat FROM shape_nodes WHERE shape_id = hashed_shape_id AND node_index = curr_idx) - 
                      (SELECT lat FROM shape_nodes WHERE shape_id = hashed_shape_id AND node_index = prev_idx),
                      (SELECT lon FROM shape_nodes WHERE shape_id = hashed_shape_id AND node_index = curr_idx) - 
                      (SELECT lon FROM shape_nodes WHERE shape_id = hashed_shape_id AND node_index = prev_idx)
                  )
              ) < (bearing_threshold_deg * (pi() / 180))
          )
    ),

    -- 4. EXPAND: Explode 1 row into multiple per-node rows
    expanded_nodes AS (
        SELECT
            hashed_shape_id,
            arrayJoin(range(prev_idx, curr_idx)) AS node_index,
            toUInt8(toHour(toDateTime(prev_ts / 1000))) AS hour,
            prev_ts AS created_at,
            time_delta / (curr_idx - prev_idx) AS travel_time_seconds,
            speed_kmh
        FROM filtered_segments
    )

-- 5. FINAL ENRICHMENT: Join with shape_nodes to get Lat/Lon for every record
SELECT 
    en.hashed_shape_id, en.node_index, en.hour,
    sn.lat, sn.lon,
    en.created_at, en.travel_time_seconds, en.speed_kmh
FROM expanded_nodes AS en
JOIN shape_nodes AS sn ON en.hashed_shape_id = sn.shape_id AND en.node_index = sn.node_index;