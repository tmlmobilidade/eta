CREATE TABLE IF NOT EXISTS shape_performance
(
    hashed_shape_id          String,
    node_count               UInt32,
    total_distance_m         Float64,
    avg_travel_time_s        Float64,
    min_hour_travel_time_s   Float64,
    max_hour_travel_time_s   Float64,
    avg_speed_kmh            Float64,
    hours_covered            UInt8,
    total_samples            UInt64
)
ENGINE = MergeTree
ORDER BY (hashed_shape_id);
