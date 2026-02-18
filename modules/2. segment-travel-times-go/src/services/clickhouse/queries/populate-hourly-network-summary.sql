INSERT INTO hourly_network_summary
SELECT
    hour,
    uniq(hashed_shape_id)                    AS shape_count,
    count()                                  AS total_nodes,
    avg(travel_time_seconds)                 AS avg_travel_time_s,
    quantile(0.5)(travel_time_seconds)       AS p50_travel_time_s,
    quantile(0.9)(travel_time_seconds)       AS p90_travel_time_s,
    quantile(0.95)(travel_time_seconds)      AS p95_travel_time_s,
    if(
        avg(travel_time_seconds) > 0,
        (25.0 / avg(travel_time_seconds)) * 3.6,
        0
    )                                        AS avg_speed_kmh,
    avg(sample_count)                        AS avg_sample_count,
    sum(sample_count)                        AS total_samples
FROM node_travel_times
GROUP BY hour
ORDER BY hour;
