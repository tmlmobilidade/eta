CREATE TABLE IF NOT EXISTS hourly_network_summary
(
    hour                     UInt8,
    shape_count              UInt32,
    total_nodes              UInt64,
    avg_travel_time_s        Float64,
    p50_travel_time_s        Float64,
    p90_travel_time_s        Float64,
    p95_travel_time_s        Float64,
    avg_speed_kmh            Float64,
    avg_sample_count         Float64,
    total_samples            UInt64
)
ENGINE = MergeTree
ORDER BY (hour);
