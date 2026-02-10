package services

import (
	"context"
	"main/src/lib"
	"main/src/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *MongoClient) FetchHashedShapesByIDs(ctx context.Context, ids []string) types.HashedShapesMap {
	//

	//
	collection := c.database.Collection("hashed_shapes")
	filter := bson.M{"_id": bson.M{"$in": ids}}
	// projection := bson.M{
	// 	"_id": 1,
	// 	"points": bson.M{
	// 		"shape_pt_lat": 1,
	// 		"shape_pt_lon": 1,
	// 	},
	// }
	opts := options.Find()//.SetProjection(projection)

	//
	// Get the cursor
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		panic(lib.AppLogger.Error(err, "failed to find hashed shapes"))
	}
	defer cursor.Close(ctx)
	
	//
	// Decode results into map
	hashedShapes := make(types.HashedShapesMap, len(ids))
	for cursor.Next(ctx) {
		type Item struct {
			Id string `bson:"_id"`
			Points []types.ShapePoint `bson:"points"`
		}
		var item Item
		if err := cursor.Decode(&item); err != nil {
			panic(lib.AppLogger.Error(err, "failed to decode hashed shape"))
		}

		hashedShapes[item.Id] = item.Points
	}
	
	if err := cursor.Err(); err != nil {
		panic(lib.AppLogger.Error(err, "cursor error"))
	}

	//
	// Return the map
	return hashedShapes
}