package services

import (
	"context"
	"main/src/lib"
	"main/src/lib/geo"
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
		return nil, 0, lib.AppLogger.Error(err, "failed to count rides")
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
		return nil, 0, lib.AppLogger.Error(err, "failed to aggregate rides")
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
	pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
		"line_id":1001,
	}}})

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

// createEmptyLineShapeData creates an empty LineShapeData structure.
func createEmptyLineShapeData() *types.LineShapeData {
	return &types.LineShapeData{
		Geohashes:      make(map[string]struct{}),
		HashedShapeIDs: []string{},
		Nodes:          make(map[string][]types.Coordinate), // hashed_shape_id -> []Coordinate
	}
}

// addGeohashesFromEndpoints encodes segment endpoints to geohashes and adds them to the line data.
func addGeohashesFromEndpoints(lineData *types.LineShapeData, endpoints []types.Coordinate, geohashPrecision int) {
	for _, endpoint := range endpoints {
		hash := geo.EncodeCoordinate(endpoint, uint(geohashPrecision))
		lineData.Geohashes[hash] = struct{}{}
	}
}

// shapePointsToCoordinates converts ShapePoint slice to Coordinate slice.
func shapePointsToCoordinates(points []types.ShapePoint) []types.Coordinate {
	coords := make([]types.Coordinate, len(points))
	for i, point := range points {
		coords[i] = types.Coordinate{point.Lon, point.Lat}
	}
	return coords
}

// processRideWithHashedShape processes a single ride using pre-fetched hashed shape data.
func processRideWithHashedShape(ride *types.Ride, hashedShape types.HashedShapePointProjection, lineShapes types.LineShapesMap, settings *types.Settings) {
	// Convert ShapePoint[] to Coordinate[]
	coords := shapePointsToCoordinates(hashedShape.Points)
	
	// Process shape into segment endpoints
	segmentEndpoints := geo.ChunkLineIntoSegments(coords, settings.SegmentLengthMeters)

	// Get or create line data entry
	lineData, exists := lineShapes[ride.LineID]
	if !exists {
		lineData = createEmptyLineShapeData()
		lineShapes[ride.LineID] = lineData
	}

	// Update line data
	lineData.HashedShapeIDs = append(lineData.HashedShapeIDs, ride.HashedShapeID)
	lineData.Nodes[ride.HashedShapeID] = segmentEndpoints
	addGeohashesFromEndpoints(lineData, segmentEndpoints, settings.GeohashPrecision)
}

// AggregateRidesToLineShapes aggregates all rides into line shapes map.
// Processes rides cursor and groups shapes by line_id.
// Optimized to batch fetch all hashed shapes in a single database call.
//
// Returns the line shapes map and count of processed shapes.
func (s *MongoService) AggregateRidesToLineShapes(ctx context.Context, cursor *mongo.Cursor, settings *types.Settings, totalCount int64) (types.LineShapesMap, int, error) {
	// Step 1: Collect all rides and unique hashed_shape_ids
	var rides []*types.Ride
	uniqueHashedShapeIDs := make(map[string]struct{})

	// Create progress bar
	progressBar := lib.AppLogger.NewProgressBar(int(totalCount), "Collecting rides")

	count := 0
	for cursor.Next(ctx) {
		var ride types.Ride
		if err := cursor.Decode(&ride); err != nil {
			cursor.Close(ctx)
			if progressBar != nil {
				progressBar.Close()
			}
			return nil, 0, lib.AppLogger.Error(err, "failed to decode ride")
		}
		rides = append(rides, &ride)
		uniqueHashedShapeIDs[ride.HashedShapeID] = struct{}{}
		count++
		
		// Update progress bar
		if progressBar != nil {
			progressBar.Add()
		} else {
			// Only log debug messages when progress bar is not active
			if count % 1000 == 0 {
				lib.AppLogger.Debug("Collected %d rides with %d unique hashed shapes", count, len(uniqueHashedShapeIDs))
			}
		}
	}

	// Finish progress bar
	if progressBar != nil {
		progressBar.Finish()
	}

	if err := cursor.Err(); err != nil {
		cursor.Close(ctx)
		return nil, 0, lib.AppLogger.Error(err, "cursor error")
	}
	cursor.Close(ctx)

	lib.AppLogger.Debug("Collected %d rides with %d unique hashed shapes", len(rides), len(uniqueHashedShapeIDs))

	// Step 2: Batch fetch all hashed shapes in a single database call
	hashedShapeIDList := make([]string, 0, len(uniqueHashedShapeIDs))
	for id := range uniqueHashedShapeIDs {
		hashedShapeIDList = append(hashedShapeIDList, id)
	}

	hashedShapesMap, err := s.FetchHashedShapesByIDs(ctx, hashedShapeIDList)
	if err != nil {
		return nil, 0, err
	}

	lib.AppLogger.Debug("Fetched %d hashed shapes from database", len(hashedShapesMap))

	// Step 3: Process rides using pre-fetched hashed shapes
	processedHashedShapeIDs := make(map[string]struct{})
	lineShapes := make(types.LineShapesMap)

	for _, ride := range rides {
		// Skip if this hashed_shape_id was already processed
		if _, alreadyProcessed := processedHashedShapeIDs[ride.HashedShapeID]; alreadyProcessed {
			continue
		}

		hashedShape, exists := hashedShapesMap[ride.HashedShapeID]
		if !exists {
			continue
		}

		processedHashedShapeIDs[ride.HashedShapeID] = struct{}{}
		processRideWithHashedShape(ride, hashedShape, lineShapes, settings)
	}

	return lineShapes, len(processedHashedShapeIDs), nil
}