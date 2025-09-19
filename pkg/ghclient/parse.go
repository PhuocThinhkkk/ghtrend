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

type IssuePr struct {
	Issues       string
	PullRequests string
}

func parseIssuesPr(htmlPage string) (IssuePr, error) {
	stats := IssuePr{}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlPage))
	if err != nil {
		return stats, err
	}

	issuesSel := doc.Find(`a[data-tab-item="issues-tab"] span.Counter`)
	if issuesSel.Length() > 0 {
		stats.Issues = strings.TrimSpace(issuesSel.First().Text())
	}

	prSel := doc.Find(`a[data-tab-item="pull-requests-tab"] span.Counter`)
	if prSel.Length() > 0 {
		stats.PullRequests = strings.TrimSpace(prSel.First().Text())
	}

	return stats, nil
}

func ParseCommitCountFromHTML(html string) (int64, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return -1, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var commitCount string
	doc.Find("a[href$='/commits']").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "commit") {
			commitCount = strings.TrimSpace(s.Text())
		}
	})

	if commitCount == "" {
		return 0, nil
	}

	num, err := getNumberOfString(commitCount)
	if err != nil {
		return -1, err
	}

	return num, nil
}

func parseContributorsCountFromHTML(html string) (int64, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return -1, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var contributors string
	// Contributors link usually ends with "/contributors"
	doc.Find("a[href$='/contributors']").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "contributor") {
			contributors = strings.TrimSpace(s.Text())
		}
	})

	if contributors == "" {
		return 0, nil
	}
	num, err := getNumberOfString(contributors)
	if err != nil {
		return -1, err
	}

	return num, nil
}

func getNumberOfString(numstr string) (int64, error) {
	str := strings.Split(numstr, " ")[0]
	total := ""
	for i := len(str) - 1; i >= 0; i-- {
		s := string(str[i])
		if s == "," {
			continue
		}

		total += s
	}
	num, err := strconv.ParseInt(total, 16, 64)
	if err != nil {
		log.Println("Error when convert str into int64")
		return -1, err
	}
	return num, nil

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
