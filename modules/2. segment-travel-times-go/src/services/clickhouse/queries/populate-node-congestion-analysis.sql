INSERT INTO node_congestion_analysis
SELECT
    hashed_shape_id,
    node_index,
    any(latitude)                            AS latitude,
    any(longitude)                           AS longitude,
    avg(travel_time_seconds)                 AS avg_travel_time_s,
    max(travel_time_seconds)                 AS max_travel_time_s,
    min(travel_time_seconds)                 AS min_travel_time_s,
    toUInt8(count())                         AS hours_covered,
    sum(sample_count)                        AS total_samples,
    if(
        avg(travel_time_seconds) > 0,
        (25.0 / avg(travel_time_seconds)) * 3.6,
        0
    )                                        AS avg_speed_kmh
FROM node_travel_times
GROUP BY hashed_shape_id, node_index;
