// processor/records.go

package processor

import (
	"main/src/types"
)

// BuildRecords converts accumulators into final NodeTravelTimeRecord entries.
// Produces one record per shape + node + hour combination.
func BuildRecords(
	accumulators map[string]map[types.NodeHourKey]*types.NodeAccumulator,
	shapeNodes map[string][]types.Coordinate,
) []types.NodeTravelTimeRecord {
	var records []types.NodeTravelTimeRecord

	for shapeID, nodeAccs := range accumulators {
		nodes := shapeNodes[shapeID]
		for key, acc := range nodeAccs {
			if len(acc.Samples) == 0 || key.NodeIdx >= len(nodes) {
				continue
			}
			records = append(records, types.NodeTravelTimeRecord{
				ShapeID:           shapeID,
				NodeIndex:         key.NodeIdx,
				Hour:              key.Hour,
				Latitude:          nodes[key.NodeIdx].Latitude(),
				Longitude:         nodes[key.NodeIdx].Longitude(),
				TravelTimeSeconds: float32(acc.Median()),
				SampleCount:       uint32(len(acc.Samples)),
			})
		}
	}

	return records
}
