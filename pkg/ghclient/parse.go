package ghclient

import (
	"fmt"
	"log"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
)

var ghHrefRe = regexp.MustCompile(`^/[^/]+/[^/]+/(tree|blob)/[^/]+/.+`)

func parseTrendingPage(html string) (RepoList, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Println("error when parsing html")
		return nil, err
	}
	var repos RepoList
	doc.Find("article.Box-row").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h2 a").Text())
		owner, repoName := "", ""
		if parts := strings.Split(name, "/\n\n"); len(parts) == 2 {
			repoName = strings.ReplaceAll(parts[1], " ", "")
			owner = strings.ReplaceAll(parts[0], " ", "")
		}
		url, _ := s.Find("h2 a").Attr("href")
		description := strings.TrimSpace(s.Find("p").Text())
		lang := strings.TrimSpace(s.Find("span[itemprop='programmingLanguage']").Text())
		stars := strings.TrimSpace(s.Find("a[href$='/stargazers']").First().Text())
		forks := strings.TrimSpace(s.Find("a[href$='/network/members']").First().Text())

		repo := NewRepo(owner, repoName, lang, "https://github.com"+url, description, forks, stars)
		repos = append(repos, *repo)
	})
	return repos, nil
}

func ParseRootInfo(html string) ([]EntryInfor, error) {
	r := strings.NewReader(html)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var entries []EntryInfor

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if !strings.Contains(href, "/tree/") && !strings.Contains(href, "/blob/") {
			return
		}
		if !ghHrefRe.MatchString(href) {
			return
		}
		if seen[href] {
			return
		}
		seen[href] = true

		t := "file"
		if strings.Contains(href, "/tree/") {
			t = "dir"
		}

		name := path.Base(href)
		if name == "" || name == "." || name == "/" {
			return
		}
		name = strings.ReplaceAll(name, "%20", " ")
		name = strings.ReplaceAll(name, "%26", "&")

		entries = append(entries, EntryInfor{
			Type: t,
			Name: name,
		})
	})

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	sort.Slice(entries, func(i, j int) bool {
		priority := func(e EntryInfor) int {
			if e.Type == "dir" && strings.HasPrefix(e.Name, ".") {
				return 0
			}
			if e.Type == "dir" {
				return 1
			}
			return 2
		}

		pi := priority(entries[i])
		pj := priority(entries[j])

		if pi != pj {
			return pi < pj
		}

		return entries[i].Name < entries[j].Name
	})

	return entries, nil
}

func getReadMeHtml(htmlPage string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlPage))
	if err != nil {
		return "", err
	}

	readmeSelection := doc.Find("article.markdown-body.entry-content.container-lg")

	if readmeSelection.Length() == 0 {
		readmeSelection = doc.Find(".markdown-body")
	}

	if readmeSelection.Length() == 0 {
		return "# No README found!", nil
	}

	readmeHtml, err := readmeSelection.Html()
	if err != nil {
		return "", err
	}

	return readmeHtml, nil
}

func parseReadMeHtmlIntoMarkdown(readmeText string) (string, error) {
	markdown, err := htmltomarkdown.ConvertString(readmeText)
	if err != nil {
		return "", err
	}
	return markdown, nil
}

func parseLanguagesBreakDown(htmlPage string) (map[string]string, error) {

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlPage))
	if err != nil {
		return nil, err
	}

	langs := make(map[string]string)

	doc.Find("li.d-inline").Each(func(i int, s *goquery.Selection) {
		lang := s.Find("span.color-fg-default.text-bold.mr-1").Text()
		percent := s.Find("span").Last().Text()

		if lang != "" && percent != "" {
			langs[strings.TrimSpace(lang)] = strings.TrimSpace(percent)
		}
	})

	return langs, nil

}

// A regex that matches numbers like "63,900", "2245+", "1,234"
var numberRE = regexp.MustCompile(`[\d,]+(?:\+)?`)

func ParseCommitCountFromHTML(html string) (int64, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return -1, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Example: <a href="/golang/go/commits/master/"> ... 63,900 Commits ... </a>
	sel := doc.Find(`a[href*="/commits"]`)
	if sel.Length() == 0 {
		return 0, nil
	}

	var value int64
	found := false

	sel.EachWithBreak(func(i int, s *goquery.Selection) bool {
		// Example text: "63,900 Commits"
		txt := strings.TrimSpace(s.Text())

		if n, ok := extractLastNumber(txt); ok {
			// Example: txt = "63,900 Commits"
			// -> regex finds ["63,900"]
			// -> cleaned = "63900"
			// -> parsed = 63900
			value = n
			found = true
			return false 
		}

		childTxt := strings.TrimSpace(s.Find("span, strong").Text())
		if n, ok := extractLastNumber(childTxt); ok {
			value = n
			found = true
			return false
		}
		return true
	})

	if !found {
		return 0, nil
	}
	return value, nil
}

func extractLastNumber(s string) (int64, bool) {
	// Example s = "Contributors 2245+ 2231 contributors"
	// regex -> ["2245+", "2231"]
	matches := numberRE.FindAllString(s, -1)
	if len(matches) == 0 {
		return 0, false
	}

	for i := len(matches) - 1; i >= 0; i-- {
		tok := matches[i]        // e.g. "63,900" or "2245+"
		tok = strings.TrimSuffix(tok, "+") // remove trailing plus sign
		tok = strings.ReplaceAll(tok, ",", "") // remove commas
		if tok == "" {
			continue
		}
		n, err := strconv.ParseInt(tok, 10, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func parseContributorsCountFromHTML(html string) (int64, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return -1, fmt.Errorf("failed to parse HTML: %w", err)
	}

	sel := doc.Find(`a[href$="/contributors"]`)
	if sel.Length() == 0 {
		return 0, nil
	}

	// Get the text inside the contributors link
	text := strings.TrimSpace(sel.Text())
	if text == "" {
		return 0, nil
	}

	// Split by spaces and scan for the last pure number token
	fields := strings.Fields(text)
	var lastNum string
	for _, f := range fields {
		// remove "contributors", "+", and commas
		clean := strings.TrimSuffix(f, "contributors")
		clean = strings.ReplaceAll(clean, ",", "")
		clean = strings.TrimSuffix(clean, "+")
		if _, err := strconv.ParseInt(clean, 10, 64); err == nil {
			lastNum = clean
		}
	}

	if lastNum == "" {
		return 0, nil
	}

	num, err := strconv.ParseInt(lastNum, 10, 64)
	if err != nil {
		return -1, err
	}
	return num, nil
}




// parseIssuesPr parses the count shown in the UnderlineNav for Issues and Pull Requests.
// It tries multiple selectors and fallbacks because GitHub's attributes vary (e.g. data-tab-item may have a prefix).
func parseIssuesPr(html string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", "", err
	}

	issueSelectors := []string{
		"#issues-tab span.Counter",                            // anchor id -> span.Counter
		`a[data-tab-item$="issues-tab"] span.Counter`,        // data-tab-item ends-with "issues-tab"
		`a[id$="issues-tab"] span.Counter`,                   // anchor id ending with issues-tab
		"span#issues-repo-tab-count",                         // direct span id used in some pages
	}
	prSelectors := []string{
		"#pull-requests-tab span.Counter",
		`a[data-tab-item$="pull-requests-tab"] span.Counter`,
		`a[id$="pull-requests-tab"] span.Counter`,
		"span#pull-requests-repo-tab-count",
	}

	// helper:
	trySelectors := func(selectors []string) string {
		for _, sel := range selectors {
			if node := doc.Find(sel).First(); node.Length() > 0 {
				if txt := strings.TrimSpace(node.Text()); txt != "" {
					return txt
				}
				// fallback: 
				if title, ok := node.Attr("title"); ok && strings.TrimSpace(title) != "" {
					return strings.TrimSpace(title)
				}
			}
		}
		return ""
	}

	issues := trySelectors(issueSelectors)
	prs := trySelectors(prSelectors)

	//fallback
	if issues == "" {
		doc.Find("span.Counter").EachWithBreak(func(i int, s *goquery.Selection) bool {
			txt := strings.TrimSpace(s.Text())
			if txt != "" {
				issues = txt
				return false
			}
			return true
		})
	}
	if prs == "" {
		// try again 
		doc.Find("span.Counter").EachWithBreak(func(i int, s *goquery.Selection) bool {
			txt := strings.TrimSpace(s.Text())
			if txt != "" && txt != issues { // shit
				prs = txt
				return false
			}
			return true
		})
	}

	return issues, prs, nil
}



func NewRepo(owner string, name string, lang string, url string, description string, forks string, starts string) *Repo {
	return &Repo{
		Owner:              owner,
		Name:               name,
		Url:                url,
		Description:        description,
		Language:           lang,
		Forks:              forks,
		Stars:              starts,
		ReadMe:             "",
		Index:              -1,
		LanguagesBreakDown: map[string]string{},
		ExtraInfor:         ExtraInfor{},
		RootInfor:          []EntryInfor{},
		HtmlPageTerm:       "",
	}
}
