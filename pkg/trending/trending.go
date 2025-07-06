package trending 


import (
	"fmt"
	"strings"
	"github.com/PuerkitoBio/goquery"
)

type Repo struct {
    Index       int    `json:"index"`
    Name        string `json:"name"`
    Url         string `json:"url"`
    Description string `json:"description"`
    Language    string `json:"language"`
    Stars       string `json:"stars"`
    Forks       string `json:"forks"`
}

func ParseHtml(html string) ([]Repo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("error when parsing html")
		return nil, err
	}
	var repos    []Repo
	doc.Find("article.Box-row").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h2 a").Text())
		url, _ := s.Find("h2 a").Attr("href")
		description := strings.TrimSpace(s.Find("p").Text())
		lang := strings.TrimSpace(s.Find("span[itemprop='programmingLanguage']").Text())
		stars := strings.TrimSpace(s.Find("a[href$='/stargazers']").First().Text())
		forks := strings.TrimSpace(s.Find("a[href$='/network/members']").First().Text())

		repos = append(repos, Repo{
			Index:       i,
			Name:        name,
			Url:         "https://github.com" + url,
			Description: description,
			Language:    lang,
			Stars:       stars,
			Forks:       forks,
		})
	})
	return repos, nil
}




