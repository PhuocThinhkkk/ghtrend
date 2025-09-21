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
		t.Fatalf("Failed to fetch GitHub API: %v", err)
	}
	if err := json.Unmarshal(res, &apiEntries); err != nil {
		t.Fatalf("Failed to decode GitHub API JSON: %v", err)
	}

	url = "https://github.com/golang/go"
	resp, err := ghclient.Fetch(url)
	if err != nil {
		t.Fatalf("Failed to fetch GitHub HTML: %v", err)
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
