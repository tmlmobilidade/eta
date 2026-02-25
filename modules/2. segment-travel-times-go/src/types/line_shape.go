package types

type LineShape struct {
	// Set of unique geohashes covering all segment endpoints.
	Geohashes map[string]struct{}
	// Nodes is a map from hashed_shape_id to its segment endpoint coordinates.
	Nodes map[string][]Coordinate
}

type ShapeNode struct {
	Index int `ch:"node_index"`
	ShapeID string `ch:"shape_id"`
	Latitude float64 `ch:"lat"`
	Longitude float64 `ch:"lon"`
}

// LineShapesMap is a map structure grouping LineShapeData by line_id.
type LineShapesMap map[uint16]*LineShape