package ghclient

import (
	"log"
	"os"
	"path/filepath"
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

func getRepoHtml( owner string, repoName string ) ( string , error ) {
	url := "https://github.com/" + owner + "/" + repoName 
	repoPage, err := Fetch(url)
	if err != nil {
		return "", err
	}
	return string(repoPage), nil
}

func getRootInfor(html  string) ( []types.EntryInfor, error ){
	var entries []types.EntryInfor
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		e := fmt.Errorf("Failed to parse HTML when taking root infor: %v", err)
		return entries, e
	}

	doc.Find("div[role='row']").Each(func(i int, s *goquery.Selection) {
		log.Println("got em")
		icon, _ := s.Find("svg").Attr("aria-label")
		name := strings.TrimSpace(s.Find("a[data-testid='tree-item-file-name']").Text())

		if name == "" || (icon != "Directory" && icon != "File") {
			return
		}

		entry := types.EntryInfor{
			Name: name,
			Type: strings.ToLower(icon), 
		}
		entries = append(entries, entry)

	})

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
			repoPage, err := getRepoHtml( repo.Owner, repo.Name)
			if err != nil {
				repo.ReadMe = ""
				errChan <- err
			}
			rootInfo, err := getRootInfor(repoPage)
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

	log.Println("err when getting cache : ", err )

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



