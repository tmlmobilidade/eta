SELECT 
    line_id,
    groupArray(hashed_shape_id) AS hashed_shape_ids
FROM (
    SELECT DISTINCT line_id, hashed_shape_id 
    FROM vehicle_events
)
GROUP BY line_id;