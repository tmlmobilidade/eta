package services

import (
	"context"
	"main/src/lib"
	"main/src/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *MongoService) RidesCursor(ctx context.Context, settings *types.Settings) (*mongo.Cursor, int64, error) {
	collection := s.database.Collection("rides")
	matchFilter := buildRidesMatchFilter(settings)

	// Count with same filter used in aggregation
	totalCount, err := collection.CountDocuments(ctx, matchFilter)
	if err != nil {
		return nil, 0, lib.AppLogger.Error("failed to count rides", err.Error())
	}

	// Early return if no documents
	if totalCount == 0 {
		return nil, 0, nil
	}

	// Build and execute aggregation
	pipeline := buildRidesAggregationPipeline(settings)
	opts := options.Aggregate().SetBatchSize(100_000)

	cursor, err := collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return nil, 0, lib.AppLogger.Error("failed to aggregate rides", err.Error())
	}

	return cursor, totalCount, nil
}

var defaultAgencyIDs = []string{"41", "42", "43", "44"}

func buildRidesAggregationPipeline(settings *types.Settings) mongo.Pipeline {
	pipeline := mongo.Pipeline{
		// Match date range and agency IDs in single stage
		{{Key: "$match", Value: bson.M{
			"start_time_scheduled": bson.M{
				"$gte": settings.RideStartDate,
				"$lt":  settings.RideEndDate,
			},
			"agency_id": bson.M{"$in": defaultAgencyIDs},
		}}},
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

	// Optional debug filter
	// pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
	// 	"line_id": bson.M{
	// 		"$gte": 1001,
	// 		"$lte": 1500,
	// 	},
	// }}})

	return pipeline
}

func buildRidesMatchFilter(settings *types.Settings) bson.M {
	return bson.M{
		"start_time_scheduled": bson.M{
			"$gte": settings.RideStartDate,
			"$lt":  settings.RideEndDate,
		},
		"agency_id": bson.M{"$in": defaultAgencyIDs},
	}
}