package ghclient

import (
	"fmt"
	"testing"

)

func TestIntegration_ParseGitHubRepo(t *testing.T) {
	url := "https://github.com/golang/go"

	resp, err := Fetch(url)
	if err != nil {
		t.Fatalf("Failed to fetch GitHub HTML: %v", err)
	}
	html := string(resp)

	commits, err := ParseCommitCountFromHTML(html)
	if err != nil {
		t.Errorf("ParseCommitCountFromHTML failed: %v", err)
	} else {
		fmt.Printf("✅ Commits: %d\n", commits)
	}
	if commits <= 0 {
		t.Errorf("Expected commits > 0, got %d", commits)
	}

	contributors, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Errorf("ParseContributorsCountFromHTML failed: %v", err)
	} else {
		fmt.Printf("✅ Contributors: %d\n", contributors)
	}

	if contributors <= 0 {
		t.Errorf("Expected contributors > 0, got %d", contributors)
	}

	issues, prs, err := parseIssuesPr(html)
	if err != nil {
		t.Errorf("ParseIssuesPr failed: %v", err)
	} else {
		fmt.Printf("✅ Issues: %s | PRs: %s\n", issues, prs)
	}

	if issues == "" {
		t.Errorf("Expected issues count, got empty string")
	}
	if prs == "" {
		t.Errorf("Expected PRs count, got empty string")
	}

}

