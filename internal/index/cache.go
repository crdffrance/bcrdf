package index

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"bcrdf/pkg/utils"
)

// ChecksumCache provides in-memory caching for file checksums
type ChecksumCache struct {
	cache map[string]CacheEntry
	mutex sync.RWMutex
	stats CacheStats
}

// CacheEntry represents a cached checksum entry
type CacheEntry struct {
	Checksum   string
	Size       int64
	ModTime    time.Time
	CreatedAt  time.Time
	AccessCount int64
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int
}

// NewChecksumCache creates a new checksum cache
func NewChecksumCache() *ChecksumCache {
	return &ChecksumCache{
		cache: make(map[string]CacheEntry),
		stats: CacheStats{},
	}
}

// GetOrCompute retrieves a checksum from cache or computes it
func (cc *ChecksumCache) GetOrCompute(filePath string, fileSize int64, modTime time.Time, data []byte) string {
	cc.mutex.RLock()
	if entry, exists := cc.cache[filePath]; exists {
		// Check if cache entry is still valid
		if entry.Size == fileSize && entry.ModTime.Equal(modTime) {
			cc.stats.Hits++
			entry.AccessCount++
			cc.cache[filePath] = entry
			cc.mutex.RUnlock()
			utils.Debug("Cache HIT for: %s", filePath)
			return entry.Checksum
		}
	}
	cc.mutex.RUnlock()

	// Cache miss - compute checksum
	cc.mutex.Lock()
	defer cc.mutex.Unlock()

	// Double-check after acquiring write lock
	if entry, exists := cc.cache[filePath]; exists {
		if entry.Size == fileSize && entry.ModTime.Equal(modTime) {
			cc.stats.Hits++
			entry.AccessCount++
			cc.cache[filePath] = entry
			utils.Debug("Cache HIT (double-check) for: %s", filePath)
			return entry.Checksum
		}
	}

	// Compute new checksum
	checksum := cc.computeChecksum(data)
	
	// Store in cache
	cc.cache[filePath] = CacheEntry{
		Checksum:   checksum,
		Size:       fileSize,
		ModTime:    modTime,
		CreatedAt:  time.Now(),
		AccessCount: 1,
	}
	
	cc.stats.Misses++
	cc.stats.Size = len(cc.cache)
	
	utils.Debug("Cache MISS for: %s (computed new checksum)", filePath)
	return checksum
}

// computeChecksum computes SHA256 checksum of data
func (cc *ChecksumCache) computeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// GetStats returns cache performance statistics
func (cc *ChecksumCache) GetStats() CacheStats {
	cc.mutex.RLock()
	defer cc.mutex.RUnlock()
	return cc.stats
}

// Clear removes all entries from cache
func (cc *ChecksumCache) Clear() {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	cc.cache = make(map[string]CacheEntry)
	cc.stats.Size = 0
}

// Cleanup removes old entries to prevent memory leaks
func (cc *ChecksumCache) Cleanup(maxAge time.Duration, maxSize int) {
	cc.mutex.Lock()
	defer cc.mutex.Unlock()
	
	now := time.Now()
	removed := 0
	
	for path, entry := range cc.cache {
		// Remove old entries
		if now.Sub(entry.CreatedAt) > maxAge {
			delete(cc.cache, path)
			removed++
		}
	}
	
	// If still too large, remove least accessed entries
	if len(cc.cache) > maxSize {
		cc.removeLeastAccessed(maxSize)
	}
	
	cc.stats.Size = len(cc.cache)
	utils.Debug("Cache cleanup: removed %d entries, current size: %d", removed, len(cc.cache))
}

// removeLeastAccessed removes entries with lowest access count
func (cc *ChecksumCache) removeLeastAccessed(targetSize int) {
	// Create slice of entries for sorting
	type entryWithPath struct {
		path   string
		entry  CacheEntry
	}
	
	entries := make([]entryWithPath, 0, len(cc.cache))
	for path, entry := range cc.cache {
		entries = append(entries, entryWithPath{path, entry})
	}
	
	// Sort by access count (ascending)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].entry.AccessCount > entries[j].entry.AccessCount {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	
	// Remove least accessed entries
	toRemove := len(entries) - targetSize
	for i := 0; i < toRemove; i++ {
		delete(cc.cache, entries[i].path)
	}
} 