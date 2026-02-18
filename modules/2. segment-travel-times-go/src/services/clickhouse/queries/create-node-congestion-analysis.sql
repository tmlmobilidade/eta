CREATE TABLE IF NOT EXISTS node_congestion_analysis
(
    hashed_shape_id          String,
    node_index               Int32,
    latitude                 Float64,
    longitude                Float64,
    avg_travel_time_s        Float64,
    max_travel_time_s        Float32,
    min_travel_time_s        Float32,
    hours_covered            UInt8,
    total_samples            UInt64,
    avg_speed_kmh            Float64
)
ENGINE = MergeTree
ORDER BY (hashed_shape_id, node_index);
