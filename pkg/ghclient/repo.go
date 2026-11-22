package ghclient

import (
	"sync"

	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
)

type Repo struct {
	Index              int               `json:"index"`
	Name               string            `json:"name"`
	Owner              string            `json:"owner"`
	Url                string            `json:"url"`
	Description        string            `json:"description"`
	Language           string            `json:"language"`
	Stars              string            `json:"stars"`
	Forks              string            `json:"forks"`
	ReadMe             string            `json:"readme"`
	RootInfor          []EntryInfor      `json:"root_infor"`
	ExtraInfor         ExtraInfor        `json:"extra_info"`
	LanguagesBreakDown map[string]string `json:"language_break_down"`
	HtmlPageTerm       string            `json:"html_page_term"`
}

type ExtraInfor struct {
	Issues       string `json:"open_issues"`
	PullRequests string `json:"pull_request"`
	Contributors int64  `json:"contributors"`
	TotalCommits int64  `json:"total_commits"`
}

type EntryInfor struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type RepoList []Repo

func (repos RepoList) loadDetails() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 5*len(repos)+1)

	for i := range repos {
		signal := make(chan struct{})
		repo := &repos[i]

		wg.Add(5)
		go repo.loadHtmlPageTerm(errChan, &wg, signal)
		go repo.loadReadMe(errChan, &wg, signal)
		go repo.loadRootInfo(errChan, &wg, signal)
		go repo.loadExtraInfo(errChan, &wg, signal)
		go repo.loadLanguageBreakdown(errChan, &wg, signal)
	}
	wg.Wait()

	for i := range repos {
		repos[i].HtmlPageTerm = ""
	}

	close(errChan)
	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}

func sendErrorNonBlocking(errChan chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errChan <- err:
	default:
	}
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
		sendErrorNonBlocking(errChan, err)
		return
	}
	r.RootInfor = rootInfo
}

func (r *Repo) loadExtraInfo(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	if r.HtmlPageTerm == "" {
		return
	}
	issues, prs, err := parseIssuesPr(r.HtmlPageTerm)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}
	contributors, err := parseContributorsCountFromHTML(r.HtmlPageTerm)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}
	commits, err := ParseCommitCountFromHTML(r.HtmlPageTerm)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}

	extraInfo := ExtraInfor{
		Issues:       issues,
		PullRequests: prs,
		Contributors: contributors,
		TotalCommits: commits,
	}
	r.ExtraInfor = extraInfo
}

func (r *Repo) loadLanguageBreakdown(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	if r.HtmlPageTerm == "" {
		return
	}
	langs, err := parseLanguagesBreakDown(r.HtmlPageTerm)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}
	r.LanguagesBreakDown = langs
}

func (r *Repo) loadReadMe(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal
	if r.HtmlPageTerm == "" {
		return
	}
	readmeText, err := getReadMeHtml(r.HtmlPageTerm)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}
	readme, err := parseReadMeHtmlIntoMarkdown(readmeText)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		return
	}
	readme = utils.CleanMarkdown(readme)
	r.ReadMe = readme
}

func (r *Repo) loadHtmlPageTerm(errChan chan<- error, wg *sync.WaitGroup, signal chan<- struct{}) {
	defer wg.Done()
	url := "https://github.com/" + r.Owner + "/" + r.Name
	res, err := Fetch(url)
	if err != nil {
		sendErrorNonBlocking(errChan, err)
		close(signal)
		return
	}
	r.HtmlPageTerm = string(res)
	close(signal)
}
