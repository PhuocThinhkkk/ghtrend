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

	contributors, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Errorf("ParseContributorsCountFromHTML failed: %v", err)
	} else {
		fmt.Printf("✅ Contributors: %d\n", contributors)
	}

	issues, prs, err := parseIssuesPr(html)
	if err != nil {
		t.Errorf("ParseIssuesPr failed: %v", err)
	} else {
		fmt.Printf("✅ Issues: %s | PRs: %s\n", issues, prs)
	}
}

