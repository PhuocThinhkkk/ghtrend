package cache

import (
	"encoding/json"
	"fmt"
	"ghtrend/pkg/configs/flags"
	"ghtrend/pkg/ghclient"
	"os"
	"path/filepath"
	"time"
)

type CacheData struct {
	Timestamp int64           `json:"timestamp"`
	Since     flags.Frequency `json:"since"`    
	Language  string          `json:"language"`
	Data      []ghclient.Repo `json:"data"`
}

func SaveCache(repos []ghclient.Repo, path string, cfg *flags.CmdConfig) error {
	timestamp := time.Now().Unix()
	os.MkdirAll(filepath.Dir(path), 0755)

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
		Data:      repos,
	}

	found := false
	for i, c := range allCache {
		if c.Since == cfg.Since && c.Language == cfg.Language {
			allCache[i] = newCache
			found = true
			break
		}
	}
	if !found {
		allCache = append(allCache, newCache)
	}

	f, err = os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(allCache)
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
		if c.Since == cfg.Since && c.Language == cfg.Language {
			if time.Now().Unix()-c.Timestamp >= int64(time.Hour.Seconds()) {
				return nil, fmt.Errorf("cache miss cause of more than 60 minutes")
			}
			return c.Data, nil
		}
	}

	return nil, fmt.Errorf("cache miss for since=%s and language=%s", cfg.Since, cfg.Language)
}
