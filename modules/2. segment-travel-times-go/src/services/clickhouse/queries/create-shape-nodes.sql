CREATE TABLE shape_nodes (
    shape_id String,
    node_index UInt32,
    lat Float64,
    lon Float64
) ENGINE = ReplacingMergeTree()
ORDER BY (shape_id, node_index);