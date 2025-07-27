package httpRequest

import (
	"io"
	"sync"
	"net/http"
	"fmt"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"ghtrend/pkg/types"
)
type RepoList []types.Repo

func Fetch(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func ParseHtml(html string) (RepoList, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("error when parsing html")
		return nil, err
	}
	var repos    RepoList
	doc.Find("article.Box-row").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h2 a").Text())
		owner, repoName := "", ""
		if parts := strings.Split(name, "/\n\n      "); len(parts) == 2 {
			repoName = parts[1]
			owner = parts[0]
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
			ReadMe:     "",
		})
	})
	return repos, nil
}

// https://raw.githubusercontent.com/charmbracelet/glow/master/README.md
func GetRawGithubReadmeFile( owner string, repoName string ) ( string , error ) {
	url := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/master/README.md"
	readmeText, err := Fetch(url)
	if err == nil {
		return string(readmeText), nil
	}

	url2 := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/main/README.md"
	readmeText2, err := Fetch(url2)
	if err != nil {
		return "", err
	}
	return string(readmeText2), nil
}


func (repos RepoList) appendReadmeToAllRepo() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))
	
	for i := range repos {
		repo := &repos[i]
		wg.Add(1)
		go func(repo *types.Repo) {

			defer wg.Done()
			readme, err := GetRawGithubReadmeFile( repo.Owner, repo.Name)
			if err != nil {
				repo.ReadMe = ""
				errChan <- err
			}

			repo.ReadMe = readme
		}(repo)
	}

	wg.Wait()
	close(errChan)
	if len(errChan) > 0 {
		return <-errChan // return the first error (or aggregate if needed)
	}
	return nil
}



func GetAllTrendingRepos() (RepoList, error ) {
	res, err := Fetch("https://github.com/trending")
	if err != nil{
		return nil, err
	}
	html := string(res)

	repos , err := ParseHtml(html)
	if err != nil {
		return nil, err
	}
		
	err = repos.appendReadmeToAllRepo()
	if err != nil {
		return nil, err
	}
	return repos, nil
}



