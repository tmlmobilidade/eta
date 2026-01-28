package mongodb

import (
	"context"
	"fmt"
	"log"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ShapesService provides operations for fetching hashed shapes from MongoDB.
type ShapesService struct {
	collection *mongo.Collection
}

// NewShapesService creates a new ShapesService.
func NewShapesService(client *Client) *ShapesService {
	return &ShapesService{
		collection: client.Collection("hashed_shapes"),
	}
}

// FetchHashedShapesByIDs fetches multiple hashed shapes by their IDs in a single database call.
// Returns a map of hashed shape ID to its data.
func (s *ShapesService) FetchHashedShapesByIDs(ctx context.Context, hashedShapeIDs []string) (map[string]*types.HashedShapePointProjection, error) {
	if len(hashedShapeIDs) == 0 {
		return make(map[string]*types.HashedShapePointProjection), nil
	}

	// Build query with projection
	filter := bson.M{"_id": bson.M{"$in": hashedShapeIDs}}
	projection := bson.M{
		"_id": 1,
		"points": bson.M{
			"shape_pt_lat": 1,
			"shape_pt_lon": 1,
		},
	}

	opts := options.Find().SetProjection(projection)

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find hashed shapes: %w", err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]*types.HashedShapePointProjection)
	for cursor.Next(ctx) {
		var shape types.HashedShapePointProjection
		if err := cursor.Decode(&shape); err != nil {
			return nil, fmt.Errorf("failed to decode hashed shape: %w", err)
		}
		result[shape.ID] = &shape
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	log.Printf("Fetched %d hashed shapes from database", len(result))
	return result, nil
}
