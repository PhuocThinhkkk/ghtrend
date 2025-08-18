package ghclient

import (
	"encoding/json"
	"fmt"
	"ghtrend/pkg/cache"
	"ghtrend/pkg/types"
	"ghtrend/pkg/utils"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
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


func fetch(url string) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token := os.Getenv("GITHUB_TOKEN")
	token = strings.TrimSpace(token)
	if token != "" {
		req.Header.Set("Authorization", "token " + token )
	}

	res, err := client.Do(req)
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

func getRawGithubReadmeFile( owner string, repoName string ) ( string , error ) {
	url := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/master/README.md"
	readmeText, err := fetch(url)
	if err == nil {
		return string(readmeText), nil
	}

	url2 := "https://raw.githubusercontent.com/" + owner + "/" + repoName + "/main/README.md"
	readmeText2, err := fetch(url2)
	if err != nil {
		return "", err
	}
	return string(readmeText2), nil
}


func getRootInfor(owner  string, name string) ( []types.EntryInfor, error ){
	var entries []types.EntryInfor
	url := "https://github.com/" + owner + "/" + name 
	res, err := fetch(url)
	if err != nil {
		return []types.EntryInfor{}, err
	}
	entries, err = parseRootInfo(string(res))
	if err != nil {
		return []types.EntryInfor{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		priority := func(e types.EntryInfor) int {
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

func getExtraInfor(owner string, name string ) (types.ExtraInfor, error) {
	url := "https://api.github.com/repos/" + owner + "/" + name 
	res, err := fetch(url)
	if err != nil {
		return types.ExtraInfor{}, err
	}
	var contents types.GitHubRepo
	err = json.Unmarshal(res, &contents)
	if err != nil {
		log.Fatal(err)
	}
	
	info := types.ExtraInfor{
		Size:  int16(contents.Size),
		Watchers: int16(contents.WatchersCount),
		OpenIssues: int16(contents.OpenIssuesCount),
		SubscribersCount : int16(contents.SubscribersCount),
	}	
	return info, nil
}

func getLanguageBreakDown(owner string, name string) (map[string]int, error) {
    url := "https://api.github.com/repos/" + owner + "/" + name + "/languages"
    res, err := fetch(url)
    if err != nil {
        return nil, err
    }

    var raw map[string]interface{}
    if err = json.Unmarshal(res, &raw); err != nil {
        log.Println(string(res))
        return nil, err
    }

    languages := make(map[string]int)
    for k, v := range raw {
        switch val := v.(type) {
        case float64:
            languages[k] = int(val)
        case int:
            languages[k] = val
        case string:
            if i, err := strconv.Atoi(val); err == nil {
                languages[k] = i
            } else {
                log.Printf("Warning: cannot convert value of %s to int: %v\n", k, val)
            }
        default:
            log.Printf("Warning: unknown type for key %s: %T\n", k, val)
        }
    }

    return languages, nil
}



func (repos RepoList) getFullInfor() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))
	
	for i := range repos {
		repo := &repos[i]
		wg.Add(4)

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

		go func(repo *types.Repo) {

			defer wg.Done()
			extraInfo, err := getExtraInfor( repo.Owner, repo.Name)
			if err != nil {
				repo.ExtraInfor = types.ExtraInfor{}
				errChan <- err
			}

			repo.ExtraInfor = extraInfo
		}(repo)

		go func(repo *types.Repo) {

			defer wg.Done()
			languages, err := getLanguageBreakDown( repo.Owner, repo.Name)
			if err != nil {
				errChan <- err
			}
			repo.LanguagesBreakDown = languages
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

	res, err := fetch("https://github.com/trending")
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



