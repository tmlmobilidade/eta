// Package processor implements the core travel time processing logic.
package processor

import (
	"context"
	"fmt"
	"log"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/geo"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/mongodb"
	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

// Aggregator handles the aggregation of rides into line shapes.
type Aggregator struct {
	ridesService  *mongodb.RidesService
	shapesService *mongodb.ShapesService
}

// NewAggregator creates a new Aggregator.
func NewAggregator(ridesService *mongodb.RidesService, shapesService *mongodb.ShapesService) *Aggregator {
	return &Aggregator{
		ridesService:  ridesService,
		shapesService: shapesService,
	}
}

// processShapeToSegmentEndpoints converts hashed shape points into segment endpoints.
// It chunks the line into segments of the specified length and extracts endpoints.
func processShapeToSegmentEndpoints(points []types.ShapePoint, segmentLengthMeters float64) []types.Coordinate {
	if len(points) < 2 {
		return []types.Coordinate{}
	}

	// Convert ShapePoints to Coordinates
	coords := make([]types.Coordinate, len(points))
	for i, p := range points {
		coords[i] = types.Coordinate{p.ShapePtLon, p.ShapePtLat}
	}

	// Chunk line into segments and extract endpoints
	return geo.ChunkLineIntoSegments(coords, segmentLengthMeters)
}

// addGeohashesFromEndpoints encodes segment endpoints to geohashes and adds them to the line data.
func addGeohashesFromEndpoints(lineData *types.LineShapeData, endpoints []types.Coordinate, geohashPrecision int) {
	for _, endpoint := range endpoints {
		hash := geo.EncodeCoordinate(endpoint, uint(geohashPrecision))
		lineData.Geohashes[hash] = struct{}{}
	}
}

// processRideWithHashedShape processes a single ride using pre-fetched hashed shape data.
func processRideWithHashedShape(
	ride *types.RideProjection,
	hashedShape *types.HashedShapePointProjection,
	lineShapes map[int]*types.LineShapeData,
	settings *types.Settings,
) {
	// Process shape into segment endpoints
	segmentEndpoints := processShapeToSegmentEndpoints(hashedShape.Points, settings.SegmentLengthMeters)

	if len(segmentEndpoints) == 0 {
		return
	}

	// Get or create line data entry
	lineData, exists := lineShapes[ride.LineID]
	if !exists {
		lineData = types.NewLineShapeData()
		lineShapes[ride.LineID] = lineData
	}

	// Update line data
	lineData.HashedShapeIDs = append(lineData.HashedShapeIDs, ride.HashedShapeID)
	lineData.Nodes[ride.HashedShapeID] = segmentEndpoints
	addGeohashesFromEndpoints(lineData, segmentEndpoints, settings.GeohashPrecision)
}

// AggregateRidesToLineShapes aggregates all rides into a line shapes map.
// It processes rides and groups shapes by line_id.
//
// The function:
// 1. Fetches all rides from MongoDB
// 2. Collects unique hashed shape IDs
// 3. Batch fetches all hashed shapes in a single database call
// 4. Processes rides using pre-fetched hashed shapes
func (a *Aggregator) AggregateRidesToLineShapes(ctx context.Context, settings *types.Settings) (map[int]*types.LineShapeData, int, error) {
	// Step 1: Fetch all rides
	rides, err := a.ridesService.FetchAllRides(ctx, settings)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch rides: %w", err)
	}

	// Step 2: Collect unique hashed_shape_ids
	uniqueHashedShapeIDs := make(map[string]struct{})
	for _, ride := range rides {
		uniqueHashedShapeIDs[ride.HashedShapeID] = struct{}{}
	}

	// Convert to slice
	shapeIDs := make([]string, 0, len(uniqueHashedShapeIDs))
	for id := range uniqueHashedShapeIDs {
		shapeIDs = append(shapeIDs, id)
	}

	log.Printf("Collected %d rides with %d unique hashed shapes", len(rides), len(shapeIDs))

	// Step 3: Batch fetch all hashed shapes
	hashedShapesMap, err := a.shapesService.FetchHashedShapesByIDs(ctx, shapeIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch hashed shapes: %w", err)
	}

	log.Printf("Fetched %d hashed shapes from database", len(hashedShapesMap))

	// Step 4: Process rides using pre-fetched hashed shapes
	processedHashedShapeIDs := make(map[string]struct{})
	lineShapes := make(map[int]*types.LineShapeData)

	for i := range rides {
		ride := &rides[i]

		// Skip if this hashed_shape_id was already processed
		if _, processed := processedHashedShapeIDs[ride.HashedShapeID]; processed {
			continue
		}

		hashedShape, found := hashedShapesMap[ride.HashedShapeID]
		if !found {
			continue
		}

		processedHashedShapeIDs[ride.HashedShapeID] = struct{}{}
		processRideWithHashedShape(ride, hashedShape, lineShapes, settings)
	}

	log.Printf("Grouped %d hashed shapes into %d lines", len(processedHashedShapeIDs), len(lineShapes))

	return lineShapes, len(processedHashedShapeIDs), nil
}
