package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RidesService provides operations for fetching rides from MongoDB.
type RidesService struct {
	collection *mongo.Collection
}

// NewRidesService creates a new RidesService.
func NewRidesService(client *Client) *RidesService {
	return &RidesService{
		collection: client.Collection("rides"),
	}
}

// buildRidesAggregationPipeline builds the aggregation pipeline for fetching rides.
func buildRidesAggregationPipeline(settings *types.Settings) mongo.Pipeline {
	return mongo.Pipeline{
		// Match date range
		{{Key: "$match", Value: bson.M{
			"start_time_scheduled": bson.M{
				"$gte": settings.RideStartDate,
				"$lt":  settings.RideEndDate,
			},
		}}},
		// Match agency IDs
		{{Key: "$match", Value: bson.M{
			"agency_id": bson.M{"$in": []string{"41", "42", "43", "44"}},
		}}},
		// DEBUG: Match line ID range
		// {{Key: "$match", Value: bson.M{
		// 	"line_id": bson.M{"$gte": 1001, "$lte": 1500},
		// }}},
		// Project only needed fields
		{{Key: "$project", Value: bson.M{
			"_id":                  0,
			"hashed_shape_id":      1,
			"line_id":              1,
			"start_time_scheduled": 1,
			"trip_id":              1,
			"vehicle_ids":          1,
		}}},
	}
}

// PrintPipelineJSON prints a MongoDB aggregation pipeline in shell-friendly JSON
func PrintPipelineJSON(pipeline interface{}) {
	var normalized bson.A

	switch p := pipeline.(type) {

	case mongo.Pipeline: // ✅ this is the key fix
		for _, stage := range p {
			normalized = append(normalized, stage)
		}

	case []bson.D:
		for _, stage := range p {
			normalized = append(normalized, stage)
		}

	case [][]bson.D:
		for _, group := range p {
			for _, stage := range group {
				normalized = append(normalized, stage)
			}
		}

	default:
		log.Printf("unsupported pipeline type: %T", pipeline)
		return
	}

	extJSON, err := bson.MarshalExtJSON(normalized, true, true)
	if err != nil {
		log.Printf("failed to marshal pipeline: %v", err)
		return
	}

	var pretty json.RawMessage
	_ = json.Unmarshal(extJSON, &pretty)

	out, _ := json.MarshalIndent(pretty, "", "  ")
	log.Println("Pipeline (MongoDB shell ready):")
	log.Println(string(out))
}

// FetchRidesCursor fetches rides and returns a cursor for iteration.
func (s *RidesService) FetchRidesCursor(ctx context.Context, settings *types.Settings) (*mongo.Cursor, int64, error) {
	// Get total count
	totalCount, err := s.collection.CountDocuments(ctx, bson.M{
		"start_time_scheduled": bson.M{
			"$gte": settings.RideStartDate,
			"$lt":  settings.RideEndDate,
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rides: %w", err)
	}

	// Build and execute aggregation pipeline
	pipeline := buildRidesAggregationPipeline(settings)
	opts := options.Aggregate().SetBatchSize(100_000)

	PrintPipelineJSON(pipeline)

	cursor, err := s.collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to aggregate rides: %w", err)
	}

	log.Printf("Found %d rides", totalCount)
	return cursor, totalCount, nil
}

// FetchAllRides fetches all rides matching the settings criteria.
func (s *RidesService) FetchAllRides(ctx context.Context, settings *types.Settings) ([]types.RideProjection, error) {
	cursor, _, err := s.FetchRidesCursor(ctx, settings)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rides []types.RideProjection
	if err := cursor.All(ctx, &rides); err != nil {
		return nil, fmt.Errorf("failed to decode rides: %w", err)
	}

	return rides, nil
}
