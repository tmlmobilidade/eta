package main

import (
	"context"
	"main/src/lib"
	"main/src/lib/geo"
	"main/src/processor"
	clickhouseService "main/src/services/clickhouse"
	mongoService "main/src/services/mongo"
	"main/src/types"
	"os"
	"os/signal"
	"slices"
	"syscall"
)

func initializeClients(config *lib.Config) (*clickhouseService.ClickhouseClient, *mongoService.MongoClient) {
	//
	
	//	
	// Initialize Clickhouse Client
	
	clickhouseClient, err := clickhouseService.NewClickhouseClient(config.Clickhouse)
	if err != nil {
		panic(err)
	}

	//	
	// Initialize MongoDB Client

	mongoClient, err := mongoService.NewMongoClient(config.MongoDB.URI, config.MongoDB.Database)
	if err != nil {
		panic(err)
	}

	return clickhouseClient, mongoClient
}

func main() {
	// Clear screen and initialize logger
	lib.AppLogger.Clear()
	lib.AppLogger.Init()

	// Load configuration
	config := lib.LoadConfig()
	lib.AppLogger.SetLogLevel(config.LogLevel)

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	//
	// Initialize Clients

	clickhouseClient, mongoClient := initializeClients(config)

	//
	// Fetch unique hashed shapes

	hashedShapesByLine := clickhouseClient.FetchUniqueHashedShapesIDsByLine(ctx)
	lib.AppLogger.Info("Found %d unique hashed shapes", len(hashedShapesByLine))

	//
	// Fetch hashed shapes by IDs

	lineShapesMap := processor.GenerateLineShapes(ctx, mongoClient, &hashedShapesByLine, config.Settings)
	lib.AppLogger.Info("Generated %d line shapes", len(lineShapesMap))


	//
	// Process line shapes

	for lineID, lineShape := range lineShapesMap {

		//
		// Fetch vehicle events

		geohashes := lib.SetToSlice(lineShape.Geohashes)
		vehicleEvents := clickhouseClient.FetchVehicleEvents(ctx, geohashes, config.Settings)
		lib.AppLogger.Info("Found %d vehicle events for line %d", len(vehicleEvents), lineID)

		//
		// Test vehicle events against Shape
		for _, shapeID := range hashedShapesByLine.GetShapesByLineID(lineID) {
			
			// Get shape nodes for this shape
			shapeNodes, exists := lineShape.Nodes[shapeID]

			// Skip shapes with less than 2 nodes
			if !exists || len(shapeNodes) < 2 {
				continue
			}

			// Calculate Travel Times for this shape by iterating over the vehicle events
			// We calculate groupings of the same ride id, this is so we can get a correct sequence of events
			for i := 1; i < len(vehicleEvents); i++ {
				//

				currEvent := vehicleEvents[i]
				prevEvent := vehicleEvents[i-1]

				// Skip if the current event is not the same trip as the previous event
				// This means we are on a new ride id
				if currEvent.RideId != prevEvent.RideId {
					continue
				}

				// Match events to nodes with a maximum distance threshold (in meters).
				prevEventNode, okPrev := matchEventToNode(prevEvent, shapeNodes, config.Settings.MaxNodeMatchDistanceMeters)
				currEventNode, okCurr := matchEventToNode(currEvent, shapeNodes, config.Settings.MaxNodeMatchDistanceMeters)

				// Skip if either event is too far from all nodes.
				if !okPrev || !okCurr {
					continue
				}

				// 
				// Calculate bearing of events and nodes

				eventsBearing := geo.CalculateBearing(
					types.Coordinate{prevEvent.Longitude, prevEvent.Latitude},
					types.Coordinate{currEvent.Longitude, currEvent.Latitude},
				)

				nodesBearing := geo.CalculateBearing(prevEventNode, currEventNode)

				if !geo.IsValidBearing(eventsBearing, nodesBearing, config.Settings.BearingThreshold) && prevEventNode != currEventNode {

					// ! DEVELOPMENT LOGGING ONLY
					lib.AppLogger.LogToFile("bearing_debug.log",
						"Events: \n 1: %v\n 2: %v\nNodes: \n 1: %v\n 2: %v\nEvents Bearing: %v\nNodes Bearing: %v\nNodes Index: %d, %d\nValid Bearing: %v",
						prevEvent,
						currEvent,
						prevEventNode,
						currEventNode,
						eventsBearing,
						nodesBearing,
						slices.Index(shapeNodes, prevEventNode),
						slices.Index(shapeNodes, currEventNode),
						geo.IsValidBearing(eventsBearing, nodesBearing, config.Settings.BearingThreshold),
					)
					panic("An invalid bearing was found")
					continue
				}

				//
				// Calculate travel time

			}
		}


	}
}

func  matchEventToNode(event types.VehicleEvent, nodes []types.Coordinate, maxNodeMatchDistanceMeters float64) (types.Coordinate, bool) {
	// Find the nearest node to the event and enforce an optional maximum distance limit (in meters).
	eventCoord := types.Coordinate{event.Longitude, event.Latitude}

	nearestNode := nodes[0]
	nearestDistance := geo.HaversineDistance(eventCoord, nearestNode)

	for i := 1; i < len(nodes); i++ {
		node := nodes[i]
		distance := geo.HaversineDistance(eventCoord, node)
		if distance < nearestDistance {
			nearestNode = node
			nearestDistance = distance
		}
	}

	// If a maximum distance is configured (> 0), only accept matches within that limit.
	if limit := maxNodeMatchDistanceMeters; limit > 0 && nearestDistance > limit {
		return types.Coordinate{}, false
	}

	return nearestNode, true
}