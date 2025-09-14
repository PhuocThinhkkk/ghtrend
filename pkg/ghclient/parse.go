package ghclient

import (
	"log"
	"path"
	"regexp"
	"sort"
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

func getLanguagesBreakDown(){

}

func parseLanguagesBreakDown(htmlLang string) (map[string]string, error){

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlLang))
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
		LanguagesBreakDown: map[string]int{},
		ExtraInfor:         ExtraInfor{},
		RootInfor:          []EntryInfor{},
		HtmlPageTerm:       "",
	}
}
