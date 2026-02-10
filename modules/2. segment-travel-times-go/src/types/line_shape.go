package types

type LineShape struct {
	// Set of unique geohashes covering all segment endpoints.
	Geohashes map[string]struct{}
	// Nodes is a map from hashed_shape_id to its segment endpoint coordinates.
	Nodes map[string][]Coordinate
}

// LineShapesMap is a map structure grouping LineShapeData by line_id.
type LineShapesMap map[uint16]*LineShape