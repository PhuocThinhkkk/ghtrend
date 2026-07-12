package ghclient

import (
	"fmt"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
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
		forks := strings.TrimSpace(s.Find("a[href$='/forks']").First().Text())

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
		name, _ = utils.DetectAndDecode(name)

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
		tok := matches[i]                      // e.g. "63,900" or "2245+"
		tok = strings.TrimSuffix(tok, "+")     // remove trailing plus sign
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

// fetchContributorsFragment retrieves the async contributors fragment.
// It's a variable (defaulting to Fetch) so tests can stub it out instead of
// making real network calls.
var fetchContributorsFragment = Fetch

// parseContributorsCountFromHTML extracts the total contributors count from
// a GitHub repository page. GitHub now renders the "Contributors" section
// via a deferred <include-fragment> element, so the count is not always
// present in the initial page HTML. When that's the case, this function
// fetches the fragment referenced by the include-fragment's `src` and
// extracts the count from there instead.
func parseContributorsCountFromHTML(html string) (int64, error) {
	num, ok, err := extractContributorsCount(html)
	if err != nil {
		return -1, err
	}
	if ok {
		return num, nil
	}

	fragmentURL, ok := contributorsFragmentURL(html)
	if !ok {
		return 0, nil
	}

	fragmentHTML, err := fetchContributorsFragment(fragmentURL)
	if err != nil {
		return 0, nil
	}

	num, ok, err = extractContributorsCount(string(fragmentHTML))
	if err != nil {
		return -1, err
	}
	if !ok {
		return 0, nil
	}
	return num, nil
}

// contributorsFragmentURL looks for a
// <include-fragment src="...contributors_list...">, which is how GitHub
// currently loads the contributors count asynchronously, and returns an
// absolute URL for it.
func contributorsFragmentURL(html string) (string, bool) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", false
	}

	src, exists := doc.Find(`include-fragment[src*="contributors_list"]`).First().Attr("src")
	if !exists || strings.TrimSpace(src) == "" {
		return "", false
	}

	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src, true
	}
	if !strings.HasPrefix(src, "/") {
		src = "/" + src
	}
	return "https://github.com" + src, true
}

// extractContributorsCount tries to find the contributors count within a
// blob of HTML. It first looks for the dedicated Counter badge next to the
// "Contributors" link (the current GitHub layout), then falls back to
// scanning the link text for a trailing number (older layout).
func extractContributorsCount(html string) (int64, bool, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return -1, false, fmt.Errorf("failed to parse HTML: %w", err)
	}

	link := doc.Find(`a[href$="/contributors"]`).First()
	if link.Length() == 0 {
		return 0, false, nil
	}

	if counterText := strings.TrimSpace(link.Find("span.Counter").First().Text()); counterText != "" {
		if n, ok := parseCountToken(counterText); ok {
			return n, true, nil
		}
	}

	// Split by spaces and scan for the last pure number token
	fields := strings.Fields(strings.TrimSpace(link.Text()))
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
		return 0, false, nil
	}

	num, err := strconv.ParseInt(lastNum, 10, 64)
	if err != nil {
		return -1, false, err
	}
	return num, true, nil
}

// parseCountToken cleans up a token like "2,231" or "2245+" and parses it
// into an int64.
func parseCountToken(token string) (int64, bool) {
	clean := strings.ReplaceAll(token, ",", "")
	clean = strings.TrimSuffix(clean, "+")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseIssuesPr parses the count shown in the UnderlineNav for Issues and Pull Requests.
// It tries multiple selectors and fallbacks because GitHub's attributes vary (e.g. data-tab-item may have a prefix).
func parseIssuesPr(html string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", "", err
	}

	issueSelectors := []string{
		"#issues-tab span.Counter",                    // anchor id -> span.Counter
		`a[data-tab-item$="issues-tab"] span.Counter`, // data-tab-item ends-with "issues-tab"
		`a[id$="issues-tab"] span.Counter`,            // anchor id ending with issues-tab
		"span#issues-repo-tab-count",                  // direct span id used in some pages
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
