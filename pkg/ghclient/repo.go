package ghclient

import (
	"sync"
)

type Repo struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	Url         string `json:"url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       string `json:"stars"`
	Forks       string `json:"forks"`
	ReadMe      string `json:"readme"` 
	RootInfor   []EntryInfor  `json:"root_infor"`
	ExtraInfor  ExtraInfor    `json:"extra_info"`
	LanguagesBreakDown map[string]int  `json:"language_break_down"`

}

type ExtraInfor struct {
	Size        int16    `json:"size"`
	Watchers    int16     `json:"watchers"`
	OpenIssues   int16     `json:"open_issues"`
	SubscribersCount  int16   `json:"Supscribers_count"`
}

type EntryInfor struct {
	Name   string `json:"name"`
	Type   string  `json:"type"`
}

type RepoList []Repo


func (repos RepoList) getFullInfor() error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))

	for i := range repos {
		repo := &repos[i]
		wg.Add(4)

		go func(repo * Repo) {

			defer wg.Done()
			readme, err := getRawGithubReadmeFile(repo.Owner, repo.Name)
			if err != nil {
				repo.ReadMe = ""
				errChan <- err
			}

			repo.ReadMe = readme
		}(repo)

		go func(repo *Repo) {

			defer wg.Done()
			rootInfo, err := getRootInfor(repo.Owner, repo.Name)
			if err != nil {
				repo.RootInfor = []EntryInfor{}
				errChan <- err
			}

			repo.RootInfor = rootInfo
		}(repo)

		go func(repo *Repo) {

			defer wg.Done()
			extraInfo, err := getExtraInfor(repo.Owner, repo.Name)
			if err != nil {
				repo.ExtraInfor = ExtraInfor{}
				errChan <- err
			}

			repo.ExtraInfor = extraInfo
		}(repo)

		go func(repo *Repo) {

			defer wg.Done()
			languages, err := getLanguageBreakDown(repo.Owner, repo.Name)
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

