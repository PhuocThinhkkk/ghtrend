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
