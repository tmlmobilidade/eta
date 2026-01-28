package geo

import (
	"math"

	"github.com/tmlmobilidade/segment-travel-times-go/internal/types"
)

const (
	// EarthRadiusMeters is the mean radius of Earth in meters.
	EarthRadiusMeters = 6371000.0
)

// HaversineDistance calculates the distance between two coordinates using the Haversine formula.
// Returns the distance in meters.
func HaversineDistance(from, to types.Coordinate) float64 {
	lat1 := toRadians(from.Latitude())
	lat2 := toRadians(to.Latitude())
	dLat := toRadians(to.Latitude() - from.Latitude())
	dLon := toRadians(to.Longitude() - from.Longitude())

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusMeters * c
}

// FindNearestNodeIndex finds the index of the nearest node to a given coordinate.
func FindNearestNodeIndex(eventLon, eventLat float64, nodes []types.Coordinate) int {
	eventCoord := types.Coordinate{eventLon, eventLat}
	nearestIndex := 0
	minDistance := math.MaxFloat64

	for i, node := range nodes {
		distance := HaversineDistance(eventCoord, node)
		if distance < minDistance {
			minDistance = distance
			nearestIndex = i
		}
	}

	return nearestIndex
}

// LineLength calculates the total length of a line defined by coordinates.
// Returns the length in meters.
func LineLength(coords []types.Coordinate) float64 {
	if len(coords) < 2 {
		return 0
	}

	totalLength := 0.0
	for i := 0; i < len(coords)-1; i++ {
		totalLength += HaversineDistance(coords[i], coords[i+1])
	}
	return totalLength
}

// InterpolatePoint interpolates a point along a line at a given distance from the start.
// Returns the interpolated coordinate.
func InterpolatePoint(coords []types.Coordinate, distanceMeters float64) types.Coordinate {
	if len(coords) == 0 {
		return types.Coordinate{}
	}
	if len(coords) == 1 || distanceMeters <= 0 {
		return coords[0]
	}

	traveled := 0.0
	for i := 0; i < len(coords)-1; i++ {
		segmentLength := HaversineDistance(coords[i], coords[i+1])
		if traveled+segmentLength >= distanceMeters {
			// Interpolate within this segment
			remaining := distanceMeters - traveled
			fraction := remaining / segmentLength
			return types.Coordinate{
				coords[i].Longitude() + fraction*(coords[i+1].Longitude()-coords[i].Longitude()),
				coords[i].Latitude() + fraction*(coords[i+1].Latitude()-coords[i].Latitude()),
			}
		}
		traveled += segmentLength
	}

	// Return last point if distance exceeds line length
	return coords[len(coords)-1]
}

// ChunkLineIntoSegments divides a line into segments of specified length.
// Returns the endpoint coordinates of each segment.
func ChunkLineIntoSegments(coords []types.Coordinate, segmentLengthMeters float64) []types.Coordinate {
	if len(coords) < 2 || segmentLengthMeters <= 0 {
		return []types.Coordinate{}
	}

	totalLength := LineLength(coords)
	if totalLength == 0 {
		return []types.Coordinate{}
	}

	endpoints := []types.Coordinate{}
	currentDistance := segmentLengthMeters

	for currentDistance < totalLength {
		point := InterpolatePoint(coords, currentDistance)
		endpoints = append(endpoints, point)
		currentDistance += segmentLengthMeters
	}

	// Always include the last point
	endpoints = append(endpoints, coords[len(coords)-1])

	return endpoints
}
