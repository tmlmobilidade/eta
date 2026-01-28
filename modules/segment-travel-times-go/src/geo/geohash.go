package geo

import (
	"main/src/types"

	"github.com/mmcloughlin/geohash"
)

// EncodeCoordinate encodes a coordinate to a geohash string.
// Note: geohash.EncodeWithPrecision expects (lat, lon), but Coordinate is [lon, lat].
func EncodeCoordinate(coord types.Coordinate, precision uint) string {
	return geohash.EncodeWithPrecision(coord.Latitude(), coord.Longitude(), precision)
}

// EncodeCoordinates encodes multiple coordinates to geohash strings.
func EncodeCoordinates(coords []types.Coordinate, precision uint) []string {
	hashes := make([]string, len(coords))
	for i, coord := range coords {
		hashes[i] = EncodeCoordinate(coord, precision)
	}
	return hashes
}

// EncodeCoordinatesToSet encodes coordinates and returns unique geohashes as a map (set).
func EncodeCoordinatesToSet(coords []types.Coordinate, precision uint) map[string]struct{} {
	set := make(map[string]struct{})
	for _, coord := range coords {
		hash := EncodeCoordinate(coord, precision)
		set[hash] = struct{}{}
	}
	return set
}

// DecodeGeohash decodes a geohash string to a coordinate.
// Returns [longitude, latitude] in GeoJSON order.
func DecodeGeohash(hash string) types.Coordinate {
	lat, lon := geohash.Decode(hash)
	return types.Coordinate{lon, lat}
}
