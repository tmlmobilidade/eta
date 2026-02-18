CREATE TABLE IF NOT EXISTS shape_hourly_summary
(
    hashed_shape_id          String,
    hour                     UInt8,
    node_count               UInt32,
    total_travel_time_s      Float64,
    avg_node_travel_time_s   Float64,
    min_node_travel_time_s   Float32,
    max_node_travel_time_s   Float32,
    total_distance_m         Float64,
    avg_speed_kmh            Float64,
    total_samples            UInt64
)
ENGINE = MergeTree
ORDER BY (hashed_shape_id, hour);
