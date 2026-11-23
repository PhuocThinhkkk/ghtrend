package ghclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/PhuocThinhkkk/ghtrend/pkg/utils"
)

var (
	githubAPIBaseURL  = "https://api.github.com"
	cachedGHToken     string
	cachedGHTokenOnce sync.Once

	apiHTTPClient = &http.Client{
		Timeout: 5 * time.Second,
	}
)

func getGitHubAuthToken() string {
	cachedGHTokenOnce.Do(func() {
		if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
			cachedGHToken = token
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if tokenBytes, err := exec.CommandContext(ctx, "gh", "auth", "token").Output(); err == nil {
			cachedGHToken = strings.TrimSpace(string(tokenBytes))
		}
	})
	return cachedGHToken
}

func fetchLanguagesFromAPI(owner, repo string) (map[string]string, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid owner or repo")
	}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/languages", githubAPIBaseURL, url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ghtrend")
	req.Header.Set("Accept", "application/vnd.github+json")

	if token := getGitHubAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	UpdateRateLimitFromHeaders(resp.Header)

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var byteCounts map[string]int64
	if err := json.NewDecoder(resp.Body).Decode(&byteCounts); err != nil {
		return nil, err
	}

	var total int64
	for _, count := range byteCounts {
		total += count
	}
	if total == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(byteCounts))
	for lang, count := range byteCounts {
		pct := float64(count) / float64(total) * 100.0
		result[lang] = fmt.Sprintf("%.1f%%", pct)
	}
	return result, nil
}

type Commit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Date    string `json:"date,omitempty"`
}

func fetchCommitsFromAPI(owner, repo string) ([]Commit, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid owner or repo")
	}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=15", githubAPIBaseURL, url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ghtrend")
	req.Header.Set("Accept", "application/vnd.github+json")

	if token := getGitHubAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	UpdateRateLimitFromHeaders(resp.Header)

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var data []struct {
		Sha    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var commits []Commit
	for _, item := range data {
		hash := item.Sha
		if len(hash) > 7 {
			hash = hash[:7]
		}
		msg := item.Commit.Message
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		commits = append(commits, Commit{
			Hash:    hash,
			Message: strings.TrimSpace(msg),
			Date:    item.Commit.Author.Date,
		})
	}
	return commits, nil
}

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
	Pinned             bool              `json:"pinned,omitempty"`
	UserProfile        *UserProfile      `json:"user_profile,omitempty"`
	RecentCommits      []Commit          `json:"recent_commits,omitempty"`
	DetailsLoaded      bool              `json:"details_loaded,omitempty"`
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

// LoadDetails fetches and populates Readme, RootInfo, ExtraInfo, LanguagesBreakdown,
// and RecentCommits for the repository. If DetailsLoaded is already true, it returns immediately.
func (r *Repo) LoadDetails() error {
	if r.DetailsLoaded {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 6)
	signal := make(chan struct{})

	wg.Add(6)
	go r.loadHtmlPageTerm(errChan, &wg, signal)
	go r.loadReadMe(errChan, &wg, signal)
	go r.loadRootInfo(errChan, &wg, signal)
	go r.loadExtraInfo(errChan, &wg, signal)
	go r.loadLanguageBreakdown(errChan, &wg, signal)
	go r.loadCommits(errChan, &wg, signal)

	wg.Wait()
	r.HtmlPageTerm = ""

	close(errChan)
	if len(errChan) > 0 {
		return <-errChan
	}
	r.DetailsLoaded = true
	return nil
}

func (repos RepoList) loadDetails() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos)+1)

	for i := range repos {
		repo := &repos[i]
		wg.Add(1)
		go func(r *Repo) {
			defer wg.Done()
			if err := r.LoadDetails(); err != nil {
				sendErrorNonBlocking(errChan, err)
			}
		}(repo)
	}
	wg.Wait()

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

	if r.HtmlPageTerm != "" {
		langs, err := parseLanguagesBreakDown(r.HtmlPageTerm)
		if err == nil && len(langs) > 0 {
			r.LanguagesBreakDown = langs
			return
		}
	}

	// Modern GitHub SSR renders Languages asynchronously (SkeletonText).
	// Query GitHub API for language byte breakdown.
	if r.Owner != "" && r.Name != "" {
		apiLangs, err := fetchLanguagesFromAPI(r.Owner, r.Name)
		if err == nil && len(apiLangs) > 0 {
			r.LanguagesBreakDown = apiLangs
			return
		}
	}

	// Resilient fallback: use the primary language discovered from trending / user repo list
	if r.Language != "" {
		r.LanguagesBreakDown = map[string]string{
			r.Language: "100%",
		}
	}
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

func (r *Repo) loadCommits(errChan chan<- error, wg *sync.WaitGroup, signal <-chan struct{}) {
	defer wg.Done()
	<-signal

	if r.Owner != "" && r.Name != "" {
		commits, err := fetchCommitsFromAPI(r.Owner, r.Name)
		if err == nil && len(commits) > 0 {
			r.RecentCommits = commits
			return
		}
	}

	// Fallback from HTML or default
	if r.HtmlPageTerm != "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(r.HtmlPageTerm))
		if err == nil {
			var parsed []Commit
			doc.Find("a[href*='/commit/']").Each(func(i int, s *goquery.Selection) {
				if len(parsed) >= 15 {
					return
				}
				href, _ := s.Attr("href")
				parts := strings.Split(href, "/commit/")
				if len(parts) == 2 {
					hash := parts[1]
					if len(hash) > 7 {
						hash = hash[:7]
					}
					txt := strings.TrimSpace(s.Text())
					if txt != "" && len(txt) > 3 && !strings.Contains(txt, "/") {
						parsed = append(parsed, Commit{
							Hash:    hash,
							Message: txt,
						})
					}
				}
			})
			if len(parsed) > 0 {
				r.RecentCommits = parsed
				return
			}
		}
	}
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
