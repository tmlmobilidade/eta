package types

//
// HashedShapeIdsByLine

type HashedShapeIdsByLine struct {
	LineID uint16 `ch:"line_id"`
	HashedShapeIDs []string `ch:"hashed_shape_ids"`
}

type HashedShapeIdsByLineArray []HashedShapeIdsByLine

func (h HashedShapeIdsByLineArray) GetAllHashedShapeIDs() []string {
	hashedShapeIDs := make([]string, 0)
	for _, item := range h {
		hashedShapeIDs = append(hashedShapeIDs, item.HashedShapeIDs...)
	}
	return hashedShapeIDs
}

func (h HashedShapeIdsByLineArray) GetAllLineIDs() []uint16 {
	lineIDs := make([]uint16, 0)
	for _, item := range h {
		lineIDs = append(lineIDs, item.LineID)
	}
	return lineIDs
}

func (h HashedShapeIdsByLineArray) GetShapesByLineID(lineID uint16) []string {
	hashedShapeIDs := make([]string, 0)
	for _, item := range h {
		if item.LineID == lineID {
			hashedShapeIDs = append(hashedShapeIDs, item.HashedShapeIDs...)
		}
	}
	return hashedShapeIDs
}

//
// Hashed Shapes

type HashedShapesMap map[string][]ShapePoint
type ShapePoint struct {
	Lat float64 `bson:"shape_pt_lat"`
	Lon float64 `bson:"shape_pt_lon"`
}