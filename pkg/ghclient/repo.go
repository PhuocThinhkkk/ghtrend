package ghclient

import (
	"sync"
)

type Repo struct {
	Index              int            `json:"index"`
	Name               string         `json:"name"`
	Owner              string         `json:"owner"`
	Url                string         `json:"url"`
	Description        string         `json:"description"`
	Language           string         `json:"language"`
	Stars              string         `json:"stars"`
	Forks              string         `json:"forks"`
	ReadMe             string         `json:"readme"`
	RootInfor          []EntryInfor   `json:"root_infor"`
	ExtraInfor         ExtraInfor     `json:"extra_info"`
	LanguagesBreakDown map[string]int `json:"language_break_down"`
	HtmlPageTerm       string         `json:"html_page_term"`
	IsLoadedRepoPage   chan (bool)    `json:"is_loaded_repo_page"`
}

type ExtraInfor struct {
	Size             int16 `json:"size"`
	Watchers         int16 `json:"watchers"`
	OpenIssues       int16 `json:"open_issues"`
	SubscribersCount int16 `json:"Supscribers_count"`
}

type EntryInfor struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type RepoList []Repo

func (repos RepoList) loadDetails() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))

	for i := range repos {
		repo := &repos[i]
		if repo.IsLoadedRepoPage == nil {
			repo.IsLoadedRepoPage = make(chan bool, 1)
		}
		defer close(repo.IsLoadedRepoPage)
		wg.Add(5)
		go repo.loadHtmlPageTerm(errChan, &wg)
		go repo.loadRootInfo(errChan, &wg)
		go repo.loadExtraInfo(errChan, &wg)
		go repo.loadLanguageBreakdown(errChan, &wg)
		go repo.loadReadMe(errChan, &wg)
	}
	wg.Wait()
	close(errChan)
	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}

func (r *Repo) loadRootInfo(errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	<- r.IsLoadedRepoPage
	rootInfo, err := getRootInfor(r.Owner, r.Name)
	if err != nil {
		r.RootInfor = []EntryInfor{}
		errChan <- err
		return
	}
	r.RootInfor = rootInfo
}

func (r *Repo) loadExtraInfo(errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	extraInfo, err := getExtraInfor(r.Owner, r.Name)
	if err != nil {
		r.ExtraInfor = ExtraInfor{}
		errChan <- err
		return
	}
	r.ExtraInfor = extraInfo
}

func (r *Repo) loadLanguageBreakdown(errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	isLoaded := <- r.IsLoadedRepoPage
	if isLoaded != false {
		return 
	}
	langs, err := getLanguageBreakDown(r.Owner, r.Name)
	if err != nil {
		errChan <- err
		return
	}
	r.LanguagesBreakDown = langs
}

func (r *Repo) loadReadMe(errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	isLoaded := <- r.IsLoadedRepoPage
	if isLoaded != false {
		return 
	}
	readme, err := getRawGithubReadmeFile(r.Owner, r.Name)
	if err != nil {
		errChan <- err
		return
	}
	r.ReadMe = readme
}

func (r *Repo) loadHtmlPageTerm(errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	url := "https://github.com/" + r.Owner + "/" + r.Name
	res, err := Fetch(url)
	if err != nil {
		errChan <- err
		r.IsLoadedRepoPage <- false
		return
	}
	r.HtmlPageTerm = string(res)
	r.IsLoadedRepoPage <- true
}
