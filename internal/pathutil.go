package internal

import (
	"path/filepath"
	"sync"
)

// PathCache provides caching for expanded paths to reduce redundant expansions
type PathCache struct {
	cache map[string]string
	mu    sync.RWMutex
}

// Global path cache instance
var pathCache = &PathCache{
	cache: make(map[string]string),
}

// CachedExpandPath expands a path with caching to avoid redundant operations
// This wraps ExpandUserPath to add caching
func CachedExpandPath(path string) string {
	if path == "" {
		return ""
	}
	
	// Check cache first
	pathCache.mu.RLock()
	if expanded, exists := pathCache.cache[path]; exists {
		pathCache.mu.RUnlock()
		return expanded
	}
	pathCache.mu.RUnlock()
	
	// Expand and cache
	expanded := ExpandUserPath(path)
	if expanded != "" {
		pathCache.mu.Lock()
		pathCache.cache[path] = expanded
		pathCache.mu.Unlock()
	}
	
	return expanded
}

// CachedJoinPath joins path elements and expands the result with caching
func CachedJoinPath(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	joined := filepath.Join(elem...)
	return CachedExpandPath(joined)
}

// CachedCleanPath cleans and expands a path with caching
func CachedCleanPath(path string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	return CachedExpandPath(cleaned)
}