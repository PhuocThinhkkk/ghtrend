package integration

import (
	"encoding/json"
	"github.com/PhuocThinhkkk/ghtrend/pkg/ghclient"
	"testing"
)

type GitHubEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
}

type EntryInfo = ghclient.EntryInfor

func TestParseRootInfoAgainstGitHub(t *testing.T) {
	url := "https://api.github.com/repos/golang/go/contents/"
	var apiEntries []GitHubEntry
	res, err := ghclient.Fetch(url)
	if err != nil {
		t.Skipf("Skipping integration test: GitHub API unavailable or rate limited: %v", err)
		return
	}
	if err := json.Unmarshal(res, &apiEntries); err != nil {
		t.Skipf("Skipping integration test: Failed to decode GitHub API JSON: %v", err)
		return
	}

	url = "https://github.com/golang/go"
	resp, err := ghclient.Fetch(url)
	if err != nil {
		t.Skipf("Skipping integration test: GitHub HTML unavailable: %v", err)
		return
	}

	parsedEntries, err := ghclient.ParseRootInfo(string(resp))
	if err != nil {
		t.Fatalf("parseRootInfo failed: %v", err)
	}

	parsedMap := make(map[string]bool)
	for _, e := range parsedEntries {
		parsedMap[e.Name] = true
	}

	for _, apiE := range apiEntries {
		if !parsedMap[apiE.Name] {
			t.Errorf("Entry %q (from API) not found in parsed HTML", apiE.Name)
		}
	}
}
