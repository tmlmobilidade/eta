package processor

import (
	"context"
	"main/src/lib/geo"
	mongoService "main/src/services/mongo"
	"main/src/types"
)

func GenerateLineShapes(ctx context.Context, mongoClient *mongoService.MongoClient, hashedShapesByLine *types.HashedShapeIdsByLineArray, settings *types.Settings) types.LineShapesMap {
	
	//
	// Fetch all hashed shapes

	hashedShapes := mongoClient.FetchHashedShapesByIDs(ctx, (*hashedShapesByLine).GetAllHashedShapeIDs())

	//
	// Generate line shapes

	lineShapesMap := make(types.LineShapesMap)
	for _, line := range *hashedShapesByLine {
		
		// Map shape_id nodes
		lineShape := &types.LineShape{
			Geohashes: make(map[string]struct{}),
			Nodes: make(map[string][]types.Coordinate),
			ShapeIDs: make(map[string]struct{}),
		}

		for _, shapeId := range line.HashedShapeIDs {
		
			// Chunk line into segments
			coordinates := hashedShapes.GetCoordinatesByShapeID(shapeId)
			lineShape.Nodes[shapeId] = geo.ChunkLineIntoSegments(coordinates, settings.SegmentLengthMeters)

			// Geohash nodes
			lineShape.Geohashes = geo.EncodeCoordinatesToSet(lineShape.Nodes[shapeId], uint(settings.GeohashPrecision))

			// Add shape ID to shape IDs map
			lineShape.ShapeIDs[shapeId] = struct{}{}
		}

		//
		// Add line shape to map

		lineShapesMap[line.LineID] = lineShape
	}

	return lineShapesMap
}