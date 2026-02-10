package geo

import (
	"testing"

	"main/src/lib/geo"
	"main/src/types"
)

func TestEncodeCoordinate(t *testing.T) {
	tests := []struct {
		name      string
		coord     types.Coordinate
		precision uint
		expected  string
	}{
		{
			name:      "Lisbon precision 7",
			coord:     types.Coordinate{-9.139337, 38.722252},
			precision: 7,
			expected:  "eycs210", // Geohash for Lisbon
		},
		{
			name:      "Lisbon precision 6",
			coord:     types.Coordinate{-9.139337, 38.722252},
			precision: 6,
			expected:  "eycs21",
		},
		{
			name:      "Origin precision 7",
			coord:     types.Coordinate{0, 0},
			precision: 7,
			expected:  "s000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geo.EncodeCoordinate(tt.coord, tt.precision)
			if result != tt.expected {
				t.Errorf("EncodeCoordinate(%v, %d) = %s, expected %s", tt.coord, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestEncodeCoordinates(t *testing.T) {
	coords := []types.Coordinate{
		{-9.139337, 38.722252}, // Lisbon
		{-8.611899, 41.149561}, // Porto
	}

	result := geo.EncodeCoordinates(coords, 7)

	if len(result) != len(coords) {
		t.Errorf("EncodeCoordinates() returned %d hashes, expected %d", len(result), len(coords))
	}

	// Verify each hash has the correct precision
	for i, hash := range result {
		if len(hash) != 7 {
			t.Errorf("Hash %d has length %d, expected 7", i, len(hash))
		}
	}
}

func TestEncodeCoordinatesToSet(t *testing.T) {
	// Include duplicate coordinates
	coords := []types.Coordinate{
		{-9.139337, 38.722252}, // Lisbon
		{-9.139337, 38.722252}, // Lisbon (duplicate)
		{-8.611899, 41.149561}, // Porto
	}

	result := geo.EncodeCoordinatesToSet(coords, 7)

	// Should only have 2 unique hashes
	if len(result) != 2 {
		t.Errorf("EncodeCoordinatesToSet() returned %d unique hashes, expected 2", len(result))
	}
}

func TestDecodeGeohash(t *testing.T) {
	// Encode then decode should give approximately the same coordinate
	original := types.Coordinate{-9.139337, 38.722252}
	encoded := geo.EncodeCoordinate(original, 7)
	decoded := geo.DecodeGeohash(encoded)

	// The decoded coordinate should be close to the original
	// With precision 7, error should be within ~150m
	lonDiff := original.Longitude() - decoded.Longitude()
	latDiff := original.Latitude() - decoded.Latitude()

	if lonDiff < -0.01 || lonDiff > 0.01 {
		t.Errorf("Longitude diff %v too large", lonDiff)
	}
	if latDiff < -0.01 || latDiff > 0.01 {
		t.Errorf("Latitude diff %v too large", latDiff)
	}
}
