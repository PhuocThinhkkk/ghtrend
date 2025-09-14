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
	LanguagesBreakDown map[string]string `json:"language_break_down"`
	HtmlPageTerm       string         `json:"html_page_term"`
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
	    signal := make(chan struct{})
		repo := &repos[i]
		defer func() {
			if repo.HtmlPageTerm != "" {
				repo.HtmlPageTerm = "done"
			}
		}()

		wg.Add(5)
		go repo.loadHtmlPageTerm(errChan, &wg, signal)
		go repo.loadReadMe(errChan, &wg, signal)
		go repo.loadRootInfo(errChan, &wg, signal)
		go repo.loadExtraInfo(errChan, &wg)
		go repo.loadLanguageBreakdown(errChan, &wg, signal)
	}
	wg.Wait()
	close(errChan)
	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}

func (r *Repo) loadRootInfo(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	if r.HtmlPageTerm == "" {
		return
	}
	rootInfo, err := ParseRootInfo(r.HtmlPageTerm)
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

func (r *Repo) loadLanguageBreakdown(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	langs, err := parseLanguagesBreakDown(r.HtmlPageTerm)
	if err != nil {
		errChan <- err
		return
	}
	r.LanguagesBreakDown = langs
}

func (r *Repo) loadReadMe(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	readmeText, err := getReadMeHtml(r.HtmlPageTerm)
	if err != nil {
		errChan <- err
		return
	}
	readme, err := parseReadMeHtmlIntoMarkdown(readmeText)
	if err != nil {
		errChan <- err
		return
	}
	r.ReadMe = readme
}

func (r *Repo) loadHtmlPageTerm(errChan chan<- error, wg *sync.WaitGroup, signal chan<- struct{}) {
	defer wg.Done()
	url := "https://github.com/" + r.Owner + "/" + r.Name
	res, err := Fetch(url)
	if err != nil {
		errChan <- err
		close(signal)
		return
	}
	r.HtmlPageTerm = string(res)
	close(signal)
}
