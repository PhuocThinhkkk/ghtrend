package ghclient

import (
	"log"
	"os"
	"path/filepath"
	"encoding/json"
	"io"
	"sync"
	"net/http"
	"fmt"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"ghtrend/pkg/types"
	"ghtrend/pkg/cache"
)
type RepoList []types.Repo

type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL *string `json:"download_url"` 
	Type        string `json:"type"`
	Links       struct {
		Self string `json:"self"`
		Git  string `json:"git"`
		HTML string `json:"html"`
	} `json:"_links"`
}


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

func getReposInfo(html string) (RepoList, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Println("error when parsing html")
		return nil, err
	}
	var repos    RepoList
	doc.Find("article.Box-row").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("h2 a").Text())
		owner, repoName := "", ""
		if parts := strings.Split(name,"/\n\n"); len(parts) == 2 {
			repoName = strings.ReplaceAll(parts[1], " ", "")
			owner = strings.ReplaceAll(parts[0], " ", "")
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
func getRawGithubReadmeFile( owner string, repoName string ) ( string , error ) {
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


// https://api.github.com/repos/vercel/next.js/contents
func getRootInfor(owner  string, name string) ( []types.EntryInfor, error ){
	var entries []types.EntryInfor
	url := "https://api.github.com/repos/" + owner + "/" + name + "/contents"
	res, err := Fetch(url)
	if err != nil {
		return []types.EntryInfor{}, err
	}
	var contents []GitHubContent
	err = json.Unmarshal(res, &contents)
	if err != nil {
		log.Fatal(err)
	}

	for _, c := range contents {
		entries = append(entries, types.EntryInfor {
			Name : c.Name,
			Type: c.Type,
		})
	}

	return entries, nil
}


func (repos RepoList) getFullInfor() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))
	
	for i := range repos {
		repo := &repos[i]
		wg.Add(2)

		go func(repo *types.Repo) {

			defer wg.Done()
			readme, err := getRawGithubReadmeFile( repo.Owner, repo.Name)
			if err != nil {
				repo.ReadMe = ""
				errChan <- err
			}

			repo.ReadMe = readme
		}(repo)

		go func(repo *types.Repo) {

			defer wg.Done()
			rootInfo, err := getRootInfor(repo.Owner, repo.Name)
			if err != nil {
				repo.RootInfor = []types.EntryInfor{}
				errChan <- err
			}

			repo.RootInfor = rootInfo
		}(repo)
	}
	wg.Wait()
	close(errChan)
	if len(errChan) > 0 {
		return <-errChan 
	}
	return nil
}



func GetAllTrendingRepos() (RepoList, error ) {

	cacheDir, _ := os.UserCacheDir()
	ghtrendDir := filepath.Join(cacheDir, "ghtrend")
	cachePath := filepath.Join(ghtrendDir, "cachedata.json")

	cacheRepos, err := cache.LoadCache(cachePath)
	if err == nil {
		return cacheRepos, nil
	}

	res, err := Fetch("https://github.com/trending")
	if err != nil{
		return nil, err
	}
	html := string(res)

	repos , err := getReposInfo(html)
	if err != nil {
		return nil, err
	}
		
	err = repos.getFullInfor()
	if err != nil {
		return nil, err
	}
	
	err = cache.SaveCache(repos, cachePath)
	if err != nil {
		return nil, err
	}

	return repos, nil
}



