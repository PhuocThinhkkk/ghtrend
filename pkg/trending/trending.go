package trending 


import (
	"fmt"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"ghtrend/pkg/types"
)


func ParseHtml(html string) ([]types.Repo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("error when parsing html")
		return nil, err
	}
	var repos    []types.Repo
	doc.Find("article.Box-row").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h2 a").Text())
		owner, repoName := "", ""
		if parts := strings.Split(name, "/\n\n      "); len(parts) == 2 {
			repoName = parts[1]
			for j := range len(parts[0]){
				if j == 0 {
					continue
				}
				owner += string(parts[0][j])
			}
		}
		url, _ := s.Find("h2 a").Attr("href")
		description := strings.TrimSpace(s.Find("p").Text())
		lang := strings.TrimSpace(s.Find("span[itemprop='programmingLanguage']").Text())
		stars := strings.TrimSpace(s.Find("a[href$='/stargazers']").First().Text())
		forks := strings.TrimSpace(s.Find("a[href$='/network/members']").First().Text())

		repos = append(repos, types.Repo{
			Index:       i,
			Owner:       owner,
			Name:        repoName,
			Url:         "https://github.com" + url,
			Description: description,
			Language:    lang,
			Stars:       stars,
			Forks:       forks,
		})
	})
	return repos, nil
}




