package gemini

import (
	"log/slog"
	"sync"

	"github.com/specvital/worker/internal/domain/specview"
)

// ChunkProgress represents cached progress for resumable chunk processing.
type ChunkProgress struct {
	AnchorDomains    []specview.DomainGroup
	CompletedChunks  int
	CompletedOutputs []*specview.Phase1Output
	TotalChunks      int
	TotalUsage       *specview.TokenUsage
}

// ChunkCacheKey uniquely identifies a chunked processing session.
type ChunkCacheKey struct {
	ContentHash string // hex-encoded content hash
	Language    specview.Language
	ModelID     string
}

// ChunkCache provides in-memory caching for chunk progress.
// Survives job retries within the same process.
// For cross-process persistence, implement with PostgreSQL.
type ChunkCache struct {
	mu    sync.RWMutex
	cache map[ChunkCacheKey]*ChunkProgress
}

// globalChunkCache is the singleton cache instance.
var globalChunkCache = &ChunkCache{
	cache: make(map[ChunkCacheKey]*ChunkProgress),
}

// GetGlobalChunkCache returns the global chunk cache instance.
func GetGlobalChunkCache() *ChunkCache {
	return globalChunkCache
}

// Get retrieves cached progress for the given key.
// Returns nil if no cache entry exists.
func (c *ChunkCache) Get(key ChunkCacheKey) *ChunkProgress {
	c.mu.RLock()
	defer c.mu.RUnlock()
	progress := c.cache[key]
	slog.Info("chunk cache get",
		"content_hash", key.ContentHash[:16]+"...",
		"language", key.Language,
		"model_id", key.ModelID,
		"found", progress != nil,
		"cache_size", len(c.cache),
	)
	return progress
}

// Save stores progress for the given key.
func (c *ChunkCache) Save(key ChunkCacheKey, progress *ChunkProgress) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = progress
	slog.Info("chunk cache save",
		"content_hash", key.ContentHash[:16]+"...",
		"language", key.Language,
		"model_id", key.ModelID,
		"completed_chunks", progress.CompletedChunks,
		"cache_size", len(c.cache),
	)
}

// Delete removes the cache entry for the given key.
// Called after successful completion.
func (c *ChunkCache) Delete(key ChunkCacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, key)
}

// Clear removes all cache entries.
// Useful for testing.
func (c *ChunkCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[ChunkCacheKey]*ChunkProgress)
}
