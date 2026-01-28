package services

import (
	"context"
	"main/src/lib"
	"main/src/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *MongoService) FetchHashedShapesByIDs(ctx context.Context, hashedShapeIDs []string) (types.HashedShapePointProjectionMap, error) {
	//

	//
	collection := s.database.Collection("hashed_shapes")
	filter := bson.M{"_id": bson.M{"$in": hashedShapeIDs}}
	projection := bson.M{
		"_id": 1,
		"points": bson.M{
			"shape_pt_lat": 1,
			"shape_pt_lon": 1,
		},
	}
	opts := options.Find().SetProjection(projection)

	//
	// Get the cursor
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, lib.AppLogger.Error("failed to find hashed shapes", err.Error())
	}
	defer cursor.Close(ctx)
	
	//
	// Decode results into map
	hashedShapes := make(types.HashedShapePointProjectionMap, len(hashedShapeIDs))
	for cursor.Next(ctx) {
		var shape types.HashedShapePointProjection
		if err := cursor.Decode(&shape); err != nil {
			return nil, lib.AppLogger.Error("failed to decode hashed shape", err.Error())
		}
		hashedShapes[shape.ID] = shape
	}
	
	if err := cursor.Err(); err != nil {
		return nil, lib.AppLogger.Error("cursor error", err.Error())
	}

	//
	// Return the map
	return hashedShapes, nil
}