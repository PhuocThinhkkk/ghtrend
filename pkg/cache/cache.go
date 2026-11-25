package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
)

const cacheTTLSeconds = int64(time.Hour / time.Second)

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

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	encodeErr := json.NewEncoder(tmpFile).Encode(allCache)
	closeErr := tmpFile.Close()
	if encodeErr != nil {
		_ = os.Remove(tmpPath)
		return encodeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if renameErr := os.Rename(tmpPath, path); renameErr != nil {
			_ = os.Remove(tmpPath)
			return renameErr
		}
	}
	return nil
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
		return nil, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read existing cache %q: %w", path, err)
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
		return CacheData{}, err
	}
	defer f.Close()

	var allCache []CacheData
	if err := json.NewDecoder(f).Decode(&allCache); err != nil {
		return CacheData{}, err
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
