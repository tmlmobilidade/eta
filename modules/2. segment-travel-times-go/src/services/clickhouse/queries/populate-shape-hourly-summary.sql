INSERT INTO shape_hourly_summary
SELECT
    hashed_shape_id,
    hour,
    count()                                  AS node_count,
    sum(travel_time_seconds)                 AS total_travel_time_s,
    avg(travel_time_seconds)                 AS avg_node_travel_time_s,
    min(travel_time_seconds)                 AS min_node_travel_time_s,
    max(travel_time_seconds)                 AS max_node_travel_time_s,
    count() * 25.0                           AS total_distance_m,
    if(
        sum(travel_time_seconds) > 0,
        (count() * 25.0 / sum(travel_time_seconds)) * 3.6,
        0
    )                                        AS avg_speed_kmh,
    sum(sample_count)                        AS total_samples
FROM node_travel_times
GROUP BY hashed_shape_id, hour;
