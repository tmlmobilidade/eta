package types

// HashedShapePointProjection represents a hashed shape with only ID and point coordinates
type HashedShapePointProjectionMap map[string]HashedShapePointProjection

type HashedShapePointProjection struct {
	ID     string       `bson:"_id"`
	Points []ShapePoint `bson:"points"`
}

type ShapePoint struct {
	Lat float64 `bson:"shape_pt_lat"`
	Lon float64 `bson:"shape_pt_lon"`
}