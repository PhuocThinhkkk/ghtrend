package types

type GitHubRepo struct {
	ID               int64    `json:"id"`
	NodeID           string   `json:"node_id"`
	Name             string   `json:"name"`
	FullName         string   `json:"full_name"`
	Private          bool     `json:"private"`
	Owner            Owner    `json:"owner"`
	HTMLURL          string   `json:"html_url"`
	Description      string   `json:"description"`
	Fork             bool     `json:"fork"`
	URL              string   `json:"url"`
	LanguagesURL     string   `json:"languages_url"`
	ContributorsURL  string   `json:"contributors_url"`
	CommitsURL       string   `json:"commits_url"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	PushedAt         string   `json:"pushed_at"`
	Size             int      `json:"size"`
	Language         string   `json:"language"`
	ForksCount       int      `json:"forks_count"`
	WatchersCount    int      `json:"watchers_count"`
	Archived         bool     `json:"archived"`
	Disabled         bool     `json:"disabled"`
	OpenIssuesCount  int      `json:"open_issues_count"`
	License          License  `json:"license"`
	Topics           []string `json:"topics"`
	Visibility       string   `json:"visibility"`
	DefaultBranch    string   `json:"default_branch"`
	Organization     Owner    `json:"organization"`
	NetworkCount     int      `json:"network_count"`
	SubscribersCount int      `json:"subscribers_count"`
}

type Owner struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}

type License struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	SPDXID string `json:"spdx_id"`
	URL    string `json:"url"`
	NodeID string `json:"node_id"`
}
