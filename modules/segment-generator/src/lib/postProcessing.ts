/* * */

import { gridRing } from 'h3-js';

/* * */

/**
 * Remove hexagons that have no neighbors in the result set
 *
 * @param h3Indices - List of H3 indices
 * @param verbose - Whether to log removed count
 * @returns H3 indices with isolated hexagons removed
 */
export function removeIsolatedHexagons(h3Indices: string[], verbose = true): string[] {
	if (h3Indices.length <= 1) {
		return h3Indices;
	}

	const h3Set = new Set(h3Indices);
	const result: string[] = [];
	let isolatedCount = 0;

	for (const h3Idx of h3Indices) {
		// Get the 6 immediate neighbors
		const neighbors = gridRing(h3Idx, 1);

		// Check if any neighbor is in our set
		const hasNeighbor = neighbors.some(neighbor => h3Set.has(neighbor));

		if (hasNeighbor) {
			result.push(h3Idx);
		}
		else {
			isolatedCount++;
		}
	}

	if (isolatedCount > 0 && verbose) {
		console.log(`  Removed ${isolatedCount} isolated hexagons`);
	}

	return result;
}

/* * */

/**
 * Find connected clusters of hexagons using BFS
 *
 * @param h3Indices - List of H3 indices
 * @returns Array of clusters, each cluster is an array of H3 indices
 */
export function findConnectedClusters(h3Indices: string[]): string[][] {
	if (h3Indices.length === 0) {
		return [];
	}

	const h3Set = new Set(h3Indices);
	const visited = new Set<string>();
	const clusters: string[][] = [];

	for (const h3Idx of h3Indices) {
		if (visited.has(h3Idx)) {
			continue;
		}

		const cluster: string[] = [];
		const stack: string[] = [h3Idx];
		visited.add(h3Idx);

		while (stack.length > 0) {
			const current = stack.pop();
			if (current === undefined) break;
			cluster.push(current);

			// Get neighbors and add unvisited ones that are in our set
			try {
				const neighbors = gridRing(current, 1);
				for (const neighbor of neighbors) {
					if (h3Set.has(neighbor) && !visited.has(neighbor)) {
						visited.add(neighbor);
						stack.push(neighbor);
					}
				}
			}
			catch {
				continue;
			}
		}

		if (cluster.length > 0) {
			clusters.push(cluster);
		}
	}

	return clusters;
}

/* * */

/**
 * Split a single cluster into sub-clusters of at most maxSize using BFS-based geographic partitioning.
 * Each sub-cluster will be spatially coherent (connected region).
 *
 * @param cluster - Array of H3 indices representing a connected cluster
 * @param maxSize - Maximum size for each sub-cluster
 * @returns Array of sub-clusters
 */
function splitClusterByProximity(cluster: string[], maxSize: number): string[][] {
	if (cluster.length <= maxSize) {
		return [cluster];
	}

	const h3Set = new Set(cluster);
	const assigned = new Set<string>();
	const subClusters: string[][] = [];

	// Pick starting seeds from unassigned hexagons
	for (const startHex of cluster) {
		if (assigned.has(startHex)) {
			continue;
		}

		// BFS from this seed, collecting up to maxSize hexagons
		const subCluster: string[] = [];
		const queue: string[] = [startHex];
		assigned.add(startHex);

		while (queue.length > 0 && subCluster.length < maxSize) {
			const current = queue.shift();
			if (current === undefined) break;
			subCluster.push(current);

			// Add unassigned neighbors from the original cluster
			try {
				const neighbors = gridRing(current, 1);
				for (const neighbor of neighbors) {
					if (h3Set.has(neighbor) && !assigned.has(neighbor) && subCluster.length + queue.length < maxSize) {
						assigned.add(neighbor);
						queue.push(neighbor);
					}
				}
			}
			catch {
				continue;
			}
		}

		// Any remaining queued items that didn't make it into this cluster
		// will be unassigned so they can be picked up by the next seed
		for (const hex of queue) {
			assigned.delete(hex);
		}

		if (subCluster.length > 0) {
			subClusters.push(subCluster);
		}
	}

	return subClusters;
}

/* * */

/**
 * Split large clusters into smaller sub-clusters using geographic proximity.
 *
 * @param h3Indices - List of H3 indices
 * @param maxSize - Maximum cluster size
 * @param verbose - Whether to log split counts
 * @returns H3 indices organized into smaller clusters (flattened)
 */
export function splitLargeClusters(h3Indices: string[], maxSize: number, verbose = true): string[][] {
	const clusters = findConnectedClusters(h3Indices);
	const result: string[][] = [];
	let splitCount = 0;

	for (const cluster of clusters) {
		if (cluster.length > maxSize) {
			const subClusters = splitClusterByProximity(cluster, maxSize);
			result.push(...subClusters);
			splitCount++;
			if (verbose) {
				console.log(`  Split cluster of ${cluster.length} into ${subClusters.length} sub-clusters`);
			}
		}
		else {
			result.push(cluster);
		}
	}

	if (splitCount > 0 && verbose) {
		console.log(`  Split ${splitCount} large clusters (maxSize: ${maxSize})`);
	}

	return result;
}

/* * */

/**
 * Filter hexagons by cluster size, keeping only clusters >= minSize
 *
 * @param h3Indices - List of H3 indices
 * @param minSize - Minimum cluster size to keep
 * @returns Filtered H3 indices
 */
export function filterByClusterSize(h3Indices: string[], minSize: number): string[] {
	if (minSize <= 1) {
		return h3Indices;
	}

	const clusters = findConnectedClusters(h3Indices);
	const kept: string[] = [];

	for (const cluster of clusters) {
		if (cluster.length >= minSize) {
			kept.push(...cluster);
		}
	}

	return kept;
}

/* * */

/**
 * Filter GeoJSON features to keep only clusters with at least minSize features.
 * Features are grouped by segment_index, then filtered by geographic connectivity.
 *
 * @param geojsonData - GeoJSON FeatureCollection
 * @param minSize - Minimum cluster size
 * @returns Filtered GeoJSON FeatureCollection
 */
export function filterGeoJSONByClusterSize(
	geojsonData: GeoJSON.FeatureCollection<GeoJSON.Geometry>,
	minSize = 3,
): GeoJSON.FeatureCollection<GeoJSON.Geometry> {
	const features = geojsonData.features;
	console.log(`Filtering ${features.length} features by cluster size (min: ${minSize})...`);

	// Group features by segment_index
	const featuresBySegment = new Map<null | number, GeoJSON.Feature<GeoJSON.Geometry>[]>();
	for (const feature of features) {
		const segmentIndex = feature.properties.segment_index as null | number | undefined;
		if (segmentIndex !== undefined) {
			const key = segmentIndex;
			if (!featuresBySegment.has(key)) {
				featuresBySegment.set(key, []);
			}
			const group = featuresBySegment.get(key);
			if (group) group.push(feature);
		}
	}

	console.log(`Found ${featuresBySegment.size} unique segment_indexes`);

	// Filter each segment's clusters
	const filteredFeatures: GeoJSON.Feature<GeoJSON.Geometry>[] = [];
	let removedCount = 0;

	for (const [, segmentFeatures] of featuresBySegment) {
		const h3ToFeature = new Map<string, GeoJSON.Feature<GeoJSON.Geometry>>();
		for (const f of segmentFeatures) {
			const h3Index = f.properties.h3_index as string | undefined;
			if (h3Index) {
				h3ToFeature.set(h3Index, f);
			}
		}

		const h3Indices = Array.from(h3ToFeature.keys());
		const clusters = findConnectedClusters(h3Indices);

		for (const cluster of clusters) {
			if (cluster.length >= minSize) {
				for (const h of cluster) {
					const feature = h3ToFeature.get(h);
					if (feature) {
						filteredFeatures.push(feature);
					}
				}
			}
			else {
				removedCount += cluster.length;
			}
		}
	}

	console.log(`Kept: ${filteredFeatures.length}, Removed: ${removedCount}`);

	return { features: filteredFeatures, type: 'FeatureCollection' };
}

/* * */

/**
 * Split large clusters in GeoJSON features by geographic proximity.
 * Features are grouped by segment_index, then large clusters are split.
 * New segment_index values are assigned to split sub-clusters.
 *
 * @param geojsonData - GeoJSON FeatureCollection
 * @param maxSize - Maximum cluster size
 * @returns GeoJSON FeatureCollection with split clusters
 */
export function filterGeoJSONByMaxSize(
	geojsonData: GeoJSON.FeatureCollection<GeoJSON.Geometry>,
	maxSize: number,
): GeoJSON.FeatureCollection<GeoJSON.Geometry> {
	const features = geojsonData.features;
	console.log(`Splitting clusters larger than ${maxSize} hexagons...`);

	// Group features by segment_index
	const featuresBySegment = new Map<null | number, GeoJSON.Feature<GeoJSON.Geometry>[]>();
	for (const feature of features) {
		const segmentIndex = feature.properties.segment_index as null | number | undefined;
		const key = segmentIndex ?? null;
		if (!featuresBySegment.has(key)) {
			featuresBySegment.set(key, []);
		}
		const group = featuresBySegment.get(key);
		if (group) group.push(feature);
	}

	// Process each segment
	const resultFeatures: GeoJSON.Feature<GeoJSON.Geometry>[] = [];
	let newSegmentIndex = 0;
	let splitCount = 0;

	for (const [, segmentFeatures] of featuresBySegment) {
		const h3ToFeature = new Map<string, GeoJSON.Feature<GeoJSON.Geometry>>();
		for (const f of segmentFeatures) {
			const h3Index = f.properties.h3_index as string | undefined;
			if (h3Index) {
				h3ToFeature.set(h3Index, f);
			}
		}

		const h3Indices = Array.from(h3ToFeature.keys());
		const clusters = findConnectedClusters(h3Indices);

		for (const cluster of clusters) {
			if (cluster.length > maxSize) {
				// Split large cluster
				const subClusters = splitClusterByProximity(cluster, maxSize);
				splitCount++;
				console.log(`  Split cluster of ${cluster.length} into ${subClusters.length} sub-clusters`);

				for (const subCluster of subClusters) {
					const groupId = `group_${newSegmentIndex}`;
					for (const h of subCluster) {
						const feature = h3ToFeature.get(h);
						if (feature) {
							// Create new feature with updated segment_index, group_id, and group_size
							resultFeatures.push({
								...feature,
								properties: {
									...feature.properties,
									group_id: groupId,
									group_size: subCluster.length,
									segment_index: newSegmentIndex,
								},
							});
						}
					}
					newSegmentIndex++;
				}
			}
			else {
				// Keep cluster as-is with new segment_index
				const groupId = `group_${newSegmentIndex}`;
				for (const h of cluster) {
					const feature = h3ToFeature.get(h);
					if (feature) {
						resultFeatures.push({
							...feature,
							properties: {
								...feature.properties,
								group_id: groupId,
								group_size: cluster.length,
								segment_index: newSegmentIndex,
							},
						});
					}
				}
				newSegmentIndex++;
			}
		}
	}

	console.log(`Split ${splitCount} large clusters, total segments: ${newSegmentIndex}`);

	return { features: resultFeatures, type: 'FeatureCollection' };
}

/* * */
