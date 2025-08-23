package cache

import (
	"encoding/json"
	"fmt"
	"ghtrend/pkg/ghclient"
	"os"
	"path/filepath"
	"time"
)

type CacheData struct {
	Timestamp int64           `json:"timestamp"`
	Data      []ghclient.Repo `json:"data"`
}

func SaveCache(repos []ghclient.Repo, path string) error {
	timestamp := time.Now().Unix()
	os.MkdirAll(filepath.Dir(path), 0755)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cache := CacheData{
		Timestamp: timestamp,
		Data:      repos,
	}

	return json.NewEncoder(f).Encode(cache)
}

func LoadCache(path string) ([]ghclient.Repo, error) {
	var repos []ghclient.Repo
	var cache CacheData

	f, err := os.Open(path)
	if err != nil {
		return repos, err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&cache)
	if err != nil {
		return repos, err
	}
	if time.Now().Unix()-cache.Timestamp >= int64(time.Hour.Seconds()) {
		return repos, fmt.Errorf("cache miss cause of more than 60 minutes")
	}
	return cache.Data, err
}
