// Package geo provides geospatial utility functions for bearing, distance, and geohash calculations.
package geo

import (
	"math"

	"main/src/types"
)

const (
	// degreesToRadians is the conversion factor from degrees to radians.
	degreesToRadians = math.Pi / 180.0
	// radiansToDegrees is the conversion factor from radians to degrees.
	radiansToDegrees = 180.0 / math.Pi
)

// toRadians converts degrees to radians.
func toRadians(degrees float64) float64 {
	return degrees * degreesToRadians
}

// toDegrees converts radians to degrees.
func toDegrees(radians float64) float64 {
	return radians * radiansToDegrees
}

// CalculateBearing calculates the bearing (direction) from one coordinate to another.
// Returns bearing in degrees (0-360, where 0 is North).
func CalculateBearing(from, to types.Coordinate) float64 {
	lat1 := toRadians(from.Latitude())
	lat2 := toRadians(to.Latitude())
	dLon := toRadians(to.Longitude() - from.Longitude())

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)

	bearing := toDegrees(math.Atan2(y, x))

	// Normalize to 0-360
	if bearing < 0 {
		bearing += 360
	}
	return bearing
}

// GetAngularDifference calculates the absolute angular difference between two bearings.
// Handles wraparound at 0/360 degrees.
// Returns the absolute difference in degrees (0-180).
func GetAngularDifference(bearing1, bearing2 float64) float64 {
	diff := math.Abs(bearing1 - bearing2)
	// Handle wraparound: if diff > 180, the shorter angle is 360 - diff
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}

// IsValidBearing checks if an event bearing is valid (matches shape direction within threshold).
// An event is valid if its bearing is within the threshold of the shape bearing.
func IsValidBearing(eventBearing, shapeBearing, thresholdDegrees float64) bool {
	difference := GetAngularDifference(eventBearing, shapeBearing)
	return difference <= thresholdDegrees
}

// CalculateShapeBearings calculates bearings for consecutive shape nodes.
// Returns an array where index i contains the bearing from node[i] to node[i+1].
// The last node uses the same bearing as the previous segment.
func CalculateShapeBearings(nodes []types.Coordinate) []float64 {
	if len(nodes) < 2 {
		return []float64{}
	}

	bearings := make([]float64, len(nodes))
	for i := 0; i < len(nodes)-1; i++ {
		bearings[i] = CalculateBearing(nodes[i], nodes[i+1])
	}
	// Last node uses the previous bearing
	bearings[len(nodes)-1] = bearings[len(nodes)-2]

	return bearings
}
