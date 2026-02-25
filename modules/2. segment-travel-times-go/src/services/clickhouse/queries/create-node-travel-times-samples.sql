CREATE TABLE node_travel_times_samples (
    hashed_shape_id String,
    node_index UInt32,
    hour UInt8,
    latitude Float64,
    longitude Float64,
    created_at UInt64,
    travel_time_seconds Float32,
    speed_kmh Float64
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDateTime(created_at/1000))
ORDER BY (hashed_shape_id, hour, node_index);