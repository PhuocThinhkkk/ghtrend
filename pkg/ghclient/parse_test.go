package ghclient

import (
	"errors"
	"log"
	"testing"
)

func TestReadMe(t *testing.T) {
	markdownBody := `
    <div class="markdown-heading" dir="auto">
        <h1>html-to-markdown</h1>
    </div>
    <p> A robust html-to-markdown converter... </p>
	`
	htmlFragment := `<article class="markdown-body entry-content container-lg" itemprop="text">` + markdownBody + `</article>`

	htmlPage := "<html><body>" + htmlFragment + "</body></html>"

	htmlReadMe, err := getReadMeHtml(htmlPage)
	if err != nil {
		log.Fatal(err)
	}

	if htmlReadMe != markdownBody {
		t.Errorf("getReadMeHtml() returned %s, expected %s", htmlReadMe, markdownBody)
	}
}

func TestParseLanguage(t *testing.T) {
	html := `
	<ul>
		<li class="d-inline">
			<a>
				<span class="color-fg-default text-bold mr-1">Go</span>
				<span>85.3%</span>
			</a>
		</li>
		<li class="d-inline">
			<a>
				<span class="color-fg-default text-bold mr-1">HTML</span>
				<span>9.7%</span>
			</a>
		</li>
		<li class="d-inline">
			<a>
				<span class="color-fg-default text-bold mr-1">CSS</span>
				<span>5.0%</span>
			</a>
		</li>
	</ul>
	`

	langs, err := parseLanguagesBreakDown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"Go":   "85.3%",
		"HTML": "9.7%",
		"CSS":  "5.0%",
	}

	if len(langs) != len(expected) {
		t.Fatalf("expected %d languages, got %d", len(expected), len(langs))
	}

	for k, v := range expected {
		if langs[k] != v {
			t.Errorf("expected %s => %s, got %s", k, v, langs[k])
		}
	}

}

func TestParseIssuePr(t *testing.T) {
	html := `
	<nav>
		<a data-tab-item="issues-tab">
			Issues
			<span class="Counter" title="1,100">1.1k</span>
		</a>
		<a data-tab-item="pull-requests-tab">
			Pull requests
			<span class="Counter" title="184">184</span>
		</a>
	</nav>
	`

	Issues, PullRequests, err := parseIssuesPr(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if Issues != "1.1k" {
		t.Errorf("expected Issues = 1.1k, got %s", Issues)
	}

	if PullRequests != "184" {
		t.Errorf("expected PullRequests = 184, got %s", PullRequests)
	}
}


func TestGetCommitCountFromHTML(t *testing.T) {
	html := `
	<html>
		<body>
			<a href="/owner/repo/commits">1,234 commits</a>
		</body>
	</html>`
	html2 := `
	<html>
		<body>
			<a href="/owner/repo/commits">1,121,234 commits</a>
		</body>
	</html>`

	commits, err := ParseCommitCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	commits2, err := ParseCommitCountFromHTML(html2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := int64(1234)
	if commits != expected {
		t.Errorf("expected %d, got %d", expected, commits)
	}
	expected2 := int64(1121234)
	if commits != expected {
		t.Errorf("expected %d, got %d", expected2, commits2)
	}
}

// TestParseContributorsCountFromHTML_InlineCount covers the older GitHub
// layout, where the contributors count was rendered directly as text inside
// the "Contributors" link (no async fragment involved).
func TestParseContributorsCountFromHTML_InlineCount(t *testing.T) {
	html := `
	<html>
		<body>
			<a href="/owner/repo/contributors">2,231 contributors</a>
		</body>
	</html>`

	got, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := int64(2231)
	if got != expected {
		t.Errorf("expected %d, got %d", expected, got)
	}
}

// TestParseContributorsCountFromHTML_CounterBadge covers the current
// GitHub layout, where the "Contributors" link's text is just "Contributors"
// and the count is shown in a nested span.Counter badge.
func TestParseContributorsCountFromHTML_CounterBadge(t *testing.T) {
	html := `
	<html>
		<body>
			<h2>
				<a href="/owner/repo/graphs/contributors">Contributors
					<span class="Counter ml-1">27</span>
				</a>
			</h2>
		</body>
	</html>`

	got, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := int64(27)
	if got != expected {
		t.Errorf("expected %d, got %d", expected, got)
	}
}

// withStubbedContributorsFragment temporarily replaces the package-level
// fetchContributorsFragment hook (normally Fetch) so tests can simulate the
// async fragment response without making a real network call.
func withStubbedContributorsFragment(t *testing.T, stub func(url string) ([]byte, error)) {
	t.Helper()
	original := fetchContributorsFragment
	fetchContributorsFragment = stub
	t.Cleanup(func() { fetchContributorsFragment = original })
}

// TestParseContributorsCountFromHTML_AsyncFragment reproduces the layout
// that broke the integration test: the main page only has an empty
// "Contributors" link plus an <include-fragment> that lazily loads the
// contributors list (and its count) from a separate URL fetched at runtime.
func TestParseContributorsCountFromHTML_AsyncFragment(t *testing.T) {
	var requestedURL string
	withStubbedContributorsFragment(t, func(url string) ([]byte, error) {
		requestedURL = url
		return []byte(`
			<h2 class="h4 tmp-mb-3">
				<a href="/owner/repo/graphs/contributors" class="Link--primary">Contributors
					<span title="27" class="Counter ml-1 tmp-ml-1">27</span></a>
			</h2>
			<div class="tmp-mt-3">
				<a href="/owner/repo/graphs/contributors">+ 13 contributors</a>
			</div>
		`), nil
	})

	html := `
	<html>
		<body>
			<a href="/owner/repo/graphs/contributors">Contributors</a>
			<include-fragment src="/owner/repo/contributors_list?deferred=true"></include-fragment>
		</body>
	</html>`

	got, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := int64(27)
	if got != expected {
		t.Errorf("expected %d, got %d", expected, got)
	}

	wantURL := "https://github.com/owner/repo/contributors_list?deferred=true"
	if requestedURL != wantURL {
		t.Errorf("expected fragment fetched from %q, got %q", wantURL, requestedURL)
	}
}

// TestParseContributorsCountFromHTML_FragmentFetchFails ensures a failure to
// fetch the async fragment (e.g. network error) degrades gracefully instead
// of failing the whole parse.
func TestParseContributorsCountFromHTML_FragmentFetchFails(t *testing.T) {
	withStubbedContributorsFragment(t, func(url string) ([]byte, error) {
		return nil, errors.New("boom")
	})

	html := `
	<html>
		<body>
			<a href="/owner/repo/graphs/contributors">Contributors</a>
			<include-fragment src="/owner/repo/contributors_list?deferred=true"></include-fragment>
		</body>
	</html>`

	got, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

// TestParseContributorsCountFromHTML_NoContributorsLink ensures the function
// degrades gracefully (returns 0, no error) when there's no contributors
// link at all.
func TestParseContributorsCountFromHTML_NoContributorsLink(t *testing.T) {
	html := `<html><body><p>nothing here</p></body></html>`

	got, err := parseContributorsCountFromHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}
