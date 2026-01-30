package processor

import (
	"main/src/types"
	"sync"
)

// GeohashEventCache provides concurrent-safe caching of vehicle events by geohash.
// This allows multiple lines with overlapping geohashes to share fetched data,
// avoiding redundant ClickHouse queries.
type GeohashEventCache struct {
	mu     sync.RWMutex
	events map[string][]types.VehicleEvent // geohash -> events in that cell
}

// NewGeohashEventCache creates a new empty event cache.
func NewGeohashEventCache() *GeohashEventCache {
	return &GeohashEventCache{
		events: make(map[string][]types.VehicleEvent),
	}
}

// Get retrieves events for a specific geohash from the cache.
// Returns the events and a boolean indicating if the geohash was cached.
func (c *GeohashEventCache) Get(geohash string) ([]types.VehicleEvent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	events, ok := c.events[geohash]
	return events, ok
}

// Set stores events for a specific geohash in the cache.
func (c *GeohashEventCache) Set(geohash string, events []types.VehicleEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events[geohash] = events
}

// SetBatch stores events for multiple geohashes at once.
// The eventsByGeohash map should contain geohash -> events mappings.
func (c *GeohashEventCache) SetBatch(eventsByGeohash map[string][]types.VehicleEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for gh, events := range eventsByGeohash {
		c.events[gh] = events
	}
}

// GetCached returns which geohashes from the request are already cached.
// Returns two slices: cached geohashes and uncached geohashes.
func (c *GeohashEventCache) GetCached(geohashes []string) (cached, uncached []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached = make([]string, 0, len(geohashes))
	uncached = make([]string, 0, len(geohashes))

	for _, gh := range geohashes {
		if _, ok := c.events[gh]; ok {
			cached = append(cached, gh)
		} else {
			uncached = append(uncached, gh)
		}
	}
	return cached, uncached
}

// GetEventsForGeohashes retrieves events for multiple geohashes and combines them.
// Only returns events for geohashes that are in the cache.
func (c *GeohashEventCache) GetEventsForGeohashes(geohashes []string) []types.VehicleEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Estimate capacity: assume ~100 events per geohash on average
	allEvents := make([]types.VehicleEvent, 0, len(geohashes)*100)

	for _, gh := range geohashes {
		if events, ok := c.events[gh]; ok {
			allEvents = append(allEvents, events...)
		}
	}
	return allEvents
}

// Clear removes all cached events.
// Useful for clearing between batches to limit memory usage.
func (c *GeohashEventCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = make(map[string][]types.VehicleEvent)
}

// Size returns the number of cached geohashes.
func (c *GeohashEventCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.events)
}
