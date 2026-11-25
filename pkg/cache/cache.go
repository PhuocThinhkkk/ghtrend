package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
)

const (
	cacheTTLSeconds   = int64(time.Hour / time.Second)
	maxCacheFileBytes = 32 << 20 // 32 MiB
)

var diskCacheMu sync.Mutex

type CacheData struct {
	Timestamp int64           `json:"timestamp"`
	Since     flags.Frequency `json:"since"`
	Language  string          `json:"language"`
	User      string          `json:"user"`
	Limit     int             `json:"limit,omitempty"`
	Data      []ghclient.Repo `json:"data"`
}

func SaveCache(repos []ghclient.Repo, path string, cfg *flags.CmdConfig) error {
	if cfg == nil {
		return errors.New("cache config is nil")
	}

	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()

	timestamp := time.Now().Unix()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	allCache, err := readAllCache(path)
	if err != nil {
		return err
	}

	newCache := CacheData{
		Timestamp: timestamp,
		Since:     cfg.Since,
		Language:  cfg.Language,
		User:      cfg.User,
		Limit:     cfg.Limit,
		Data:      repos,
	}

	found := false
	for i, c := range allCache {
		if cacheKeyMatches(c, cfg) {
			allCache[i] = newCache
			found = true
			break
		}
	}
	if !found {
		allCache = append(allCache, newCache)
	}

	encoded, err := encodeCacheWithinLimit(allCache, cfg)
	if err != nil {
		return err
	}

	return writeFileAtomic(path, encoded, "cache")
}

func encodeCacheWithinLimit(allCache []CacheData, currentCfg *flags.CmdConfig) ([]byte, error) {
	encoded, err := json.Marshal(allCache)
	if err != nil {
		return nil, fmt.Errorf("encode cache data: %w", err)
	}
	if len(encoded) <= maxCacheFileBytes {
		return append(encoded, '\n'), nil
	}

	// Evict oldest non-current entries until within maxCacheFileBytes.
	var others []CacheData
	var current *CacheData
	for i := range allCache {
		if current == nil && cacheKeyMatches(allCache[i], currentCfg) {
			entry := allCache[i]
			current = &entry
			continue
		}
		others = append(others, allCache[i])
	}

	sort.Slice(others, func(i, j int) bool {
		return others[i].Timestamp > others[j].Timestamp
	})

	for len(others) > 0 {
		others = others[:len(others)-1]
		candidate := make([]CacheData, 0, len(others)+1)
		if current != nil {
			candidate = append(candidate, *current)
		}
		candidate = append(candidate, others...)
		encoded, err = json.Marshal(candidate)
		if err != nil {
			return nil, fmt.Errorf("encode trimmed cache data: %w", err)
		}
		if len(encoded) <= maxCacheFileBytes {
			return append(encoded, '\n'), nil
		}
	}

	return nil, fmt.Errorf("cache entry exceeds max allowed file size of %d bytes", maxCacheFileBytes)
}

func LoadCache(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	if cfg == nil {
		return nil, errors.New("cache config is nil")
	}

	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()

	entry, err := findCacheEntry(path, cfg)
	if err != nil {
		return nil, err
	}
	if isCacheStale(entry) {
		return nil, fmt.Errorf("cache miss cause of more than 60 minutes")
	}
	return entry.Data, nil
}

// LoadCacheWithStaleness loads matching cached repositories even if stale (>= 1 hour),
// returning whether the entry is stale so callers can implement stale-while-revalidate.
func LoadCacheWithStaleness(path string, cfg *flags.CmdConfig) (repos []ghclient.Repo, stale bool, err error) {
	if cfg == nil {
		return nil, false, errors.New("cache config is nil")
	}

	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()

	entry, err := findCacheEntry(path, cfg)
	if err != nil {
		return nil, false, err
	}
	return entry.Data, isCacheStale(entry), nil
}

func readAllCache(path string) ([]CacheData, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open existing cache %q: %w", path, err)
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxCacheFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read existing cache %q: %w", path, err)
	}
	if len(raw) > maxCacheFileBytes {
		return nil, fmt.Errorf("existing cache %q exceeded max allowed size of %d bytes", path, maxCacheFileBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}

	var allCache []CacheData
	if err := json.Unmarshal(raw, &allCache); err != nil {
		return nil, fmt.Errorf("decode existing cache %q: %w", path, err)
	}
	return allCache, nil
}

func findCacheEntry(path string, cfg *flags.CmdConfig) (CacheData, error) {
	f, err := os.Open(path)
	if err != nil {
		return CacheData{}, fmt.Errorf("open cache %q: %w", path, err)
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxCacheFileBytes+1))
	if err != nil {
		return CacheData{}, fmt.Errorf("read cache %q: %w", path, err)
	}
	if len(raw) > maxCacheFileBytes {
		return CacheData{}, fmt.Errorf("cache %q exceeded max allowed size of %d bytes", path, maxCacheFileBytes)
	}

	var allCache []CacheData
	if err := json.Unmarshal(raw, &allCache); err != nil {
		return CacheData{}, fmt.Errorf("decode cache %q: %w", path, err)
	}

	for _, c := range allCache {
		if cacheKeyMatches(c, cfg) {
			return c, nil
		}
	}

	if cfg.User != "" {
		return CacheData{}, fmt.Errorf("cache miss for user=%s and language=%s", cfg.User, cfg.Language)
	}
	return CacheData{}, fmt.Errorf("cache miss for since=%s and language=%s", cfg.Since, cfg.Language)
}

func cacheKeyMatches(c CacheData, cfg *flags.CmdConfig) bool {
	return c.Since == cfg.Since && c.Language == cfg.Language && c.User == cfg.User && c.Limit == cfg.Limit
}

func isCacheStale(c CacheData) bool {
	return time.Now().Unix()-c.Timestamp >= cacheTTLSeconds
}
