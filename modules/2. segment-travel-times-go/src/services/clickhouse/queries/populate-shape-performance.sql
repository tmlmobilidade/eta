INSERT INTO shape_performance
SELECT
    hashed_shape_id,
    node_count,
    total_distance_m,
    avg_travel_time_s,
    min_hour_travel_time_s,
    max_hour_travel_time_s,
    if(
        avg_travel_time_s > 0,
        (total_distance_m / avg_travel_time_s) * 3.6,
        0
    )                                        AS avg_speed_kmh,
    hours_covered,
    total_samples
FROM (
    SELECT
        hashed_shape_id,
        max(node_count)                          AS node_count,
        max(total_distance_m)                    AS total_distance_m,
        avg(total_travel_time_s)                 AS avg_travel_time_s,
        min(total_travel_time_s)                 AS min_hour_travel_time_s,
        max(total_travel_time_s)                 AS max_hour_travel_time_s,
        toUInt8(count())                         AS hours_covered,
        sum(total_samples)                       AS total_samples
    FROM shape_hourly_summary
    GROUP BY hashed_shape_id
);
