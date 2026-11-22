package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
)

type CacheData struct {
	Timestamp int64           `json:"timestamp"`
	Since     flags.Frequency `json:"since"`
	Language  string          `json:"language"`
	User      string          `json:"user"`
	Data      []ghclient.Repo `json:"data"`
}

func SaveCache(repos []ghclient.Repo, path string, cfg *flags.CmdConfig) error {
	timestamp := time.Now().Unix()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var allCache []CacheData

	f, err := os.Open(path)
	if err == nil {
		json.NewDecoder(f).Decode(&allCache)
		f.Close()
	}

	newCache := CacheData{
		Timestamp: timestamp,
		Since:     cfg.Since,
		Language:  cfg.Language,
		User:      cfg.User,
		Data:      repos,
	}

	found := false
	for i, c := range allCache {
		if c.Since == cfg.Since && c.Language == cfg.Language && c.User == cfg.User {
			allCache[i] = newCache
			found = true
			break
		}
	}
	if !found {
		allCache = append(allCache, newCache)
	}

	tmpPath := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

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
		return os.Rename(tmpPath, path)
	}
	return nil
}

func LoadCache(path string, cfg *flags.CmdConfig) ([]ghclient.Repo, error) {
	var allCache []CacheData

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&allCache); err != nil {
		return nil, err
	}

	for _, c := range allCache {
		if c.Since == cfg.Since && c.Language == cfg.Language && c.User == cfg.User {
			if time.Now().Unix()-c.Timestamp >= int64(time.Hour.Seconds()) {
				return nil, fmt.Errorf("cache miss cause of more than 60 minutes")
			}
			return c.Data, nil
		}
	}

	if cfg.User != "" {
		return nil, fmt.Errorf("cache miss for user=%s and language=%s", cfg.User, cfg.Language)
	}
	return nil, fmt.Errorf("cache miss for since=%s and language=%s", cfg.Since, cfg.Language)
}
