package cache 


import (
	"os"
	"fmt"
	"ghtrend/pkg/types"
	"encoding/json"
	"path/filepath"
	"time"
)



func SaveCache(repos []types.Repo, path string) error {
	timestamp := time.Now().Unix()
	os.MkdirAll(filepath.Dir(path), 0755)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	cache := types.CacheData {
		Timestamp : timestamp,
		Data : repos,
	}

	return json.NewEncoder(f).Encode(cache)
}

func LoadCache(path string) ([]types.Repo, error) {
	var repos []types.Repo
	var cache types.CacheData

	f, err := os.Open(path)
	if err != nil {
		return repos, err 
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&cache)
	if err != nil {
		return repos, err
	}
	if time.Now().Unix() - cache.Timestamp >= int64(time.Hour.Seconds()){
		return repos, fmt.Errorf("cache miss cause of more than 60 minutes")
	}
	return cache.Data, err
}


