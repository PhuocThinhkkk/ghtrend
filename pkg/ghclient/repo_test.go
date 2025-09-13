package ghclient

import (
	"log"
	"testing"
)

func TestReadMe(t *testing.T) {
	htmlFragment := `
<article class="markdown-body entry-content container-lg" itemprop="text">
    <div class="markdown-heading" dir="auto">
        <h1>html-to-markdown</h1>
    </div>
    <p> A robust html-to-markdown converter... </p>
</article>
`

	htmlPage := "<html><body>" + htmlFragment + "</body></html>"

	htmlReadMe, err := getReadMeHtml(htmlPage)
	if err != nil {
		log.Fatal(err)
	}

	if htmlReadMe != htmlFragment {
		t.Errorf("getReadMeHtml() returned %d, expected %d", htmlReadMe, htmlFragment)
	}
}
