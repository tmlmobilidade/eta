CREATE TABLE IF NOT EXISTS node_travel_times
(
    hashed_shape_id       String,
    node_index            Int32,
    hour                  UInt8,
    latitude              Float64,
    longitude             Float64,
    travel_time_seconds   Float32,
    sample_count          UInt32
)
ENGINE = MergeTree
ORDER BY (hashed_shape_id, node_index, hour);

