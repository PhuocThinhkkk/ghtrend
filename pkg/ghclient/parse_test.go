package ghclient

import (
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
