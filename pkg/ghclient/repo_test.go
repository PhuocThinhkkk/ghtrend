package ghclient

import (
	"ghtrend/pkg/utils"
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
		t.Logf("Expected: ")
		utils.LogJson(expected)
		t.Logf("Got: ")
		utils.LogJson(langs)
	}

	for i, lang := range langs {
		if lang != expected[i] || lang.Percent != expected[i].Percent {
			t.Errorf("expected %+v, got %+v", expected[i], lang)
		}
	}
}
