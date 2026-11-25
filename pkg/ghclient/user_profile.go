package ghclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PhuocThinhkkk/ghtrend/pkg/ascii"
	"github.com/PuerkitoBio/goquery"
)

type UserProfile struct {
	Username           string   `json:"username"`
	Name               string   `json:"name"`
	Bio                string   `json:"bio"`
	AvatarURL          string   `json:"avatar_url"`
	AvatarASCII        string   `json:"avatar_ascii"`
	AvatarHalfBlock    string   `json:"avatar_half_block,omitempty"`
	AvatarBraille      string   `json:"avatar_braille,omitempty"`
	Company            string   `json:"company"`
	Location           string   `json:"location"`
	Followers          string   `json:"followers"`
	Following          string   `json:"following"`
	PublicRepos        int      `json:"public_repos"`
	PinnedRepos        []string `json:"pinned_repos"`
	TotalContributions string   `json:"total_contributions,omitempty"`
	ContributionDays   []int    `json:"contribution_days,omitempty"`
	Blog               string   `json:"blog,omitempty"`
}

var (
	githubWebBaseURL      = "https://github.com"
	userProfileHTTPClient = &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: AuthSafeRedirectPolicy(),
	}
)

// GetUserProfile fetches a user's GitHub profile, extracting bio, avatar, stats, and pinned work.
// It scrapes the public profile page and falls back to the GitHub API for resilient rate-limit-free operation.
func GetUserProfile(username string) (*UserProfile, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	profile := &UserProfile{
		Username: username,
	}

	// 1. Scrape public profile page (https://github.com/<username>)
	scrapeErr := scrapeUserProfile(username, profile)

	// 2. Fall back to GitHub User API if scraping failed or avatar is missing
	var apiErr error
	if scrapeErr != nil || profile.AvatarURL == "" {
		apiErr = fetchUserProfileFromAPI(username, profile)
	}
	if scrapeErr != nil && apiErr != nil {
		return nil, fmt.Errorf("failed to fetch user profile for %q: %w", username, errors.Join(scrapeErr, apiErr))
	}

	// 3. Render avatar to ASCII, High-Resolution Half-Block, and Braille art (cached on disk)
	profile.RefreshAvatars(28, 14)

	return profile, nil
}

// RefreshAvatars re-renders the avatar in ASCII, half-block, and braille modes
// ensuring the ANSI string is freshly computed and never served from an outdated disk cache.
func (profile *UserProfile) RefreshAvatars(width, height int) {
	if profile == nil {
		return
	}
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 14
	}
	if profile.AvatarURL != "" {
		profile.AvatarASCII = ascii.AvatarToASCII(profile.AvatarURL, 22, 9)
		profile.AvatarHalfBlock = ascii.AvatarToHalfBlock(profile.AvatarURL, width, height)
		profile.AvatarBraille = ascii.AvatarToBraille(profile.AvatarURL, width, height)
	} else {
		profile.AvatarASCII = ascii.FallbackSilhouetteASCII(22, 9)
		profile.AvatarHalfBlock = ascii.FallbackSilhouetteHalfBlock(width, height)
		profile.AvatarBraille = ascii.FallbackSilhouetteBraille(width, height)
	}
}

func scrapeUserProfile(username string, profile *UserProfile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	profileURL := fmt.Sprintf("%s/%s", githubWebBaseURL, url.PathEscape(username))
	req, err := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := userProfileHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("profile scrape status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, maxAPIResponseBytes))
	if err != nil {
		return err
	}

	name := strings.TrimSpace(doc.Find("span.p-name").First().Text())
	if name != "" {
		profile.Name = name
	}
	bio := strings.TrimSpace(doc.Find("div.user-profile-bio, div.p-note").First().Text())
	if bio != "" {
		profile.Bio = bio
	}

	if avatar, exists := doc.Find("img.avatar-user").First().Attr("src"); exists && avatar != "" {
		// Prefer larger avatar size (128px)
		if strings.Contains(avatar, "s=64") {
			avatar = strings.Replace(avatar, "s=64", "s=128", 1)
		}
		profile.AvatarURL = avatar
	}

	followers := strings.TrimSpace(doc.Find("a[href$='?tab=followers'] span.text-bold").First().Text())
	if followers != "" {
		profile.Followers = followers
	}

	following := strings.TrimSpace(doc.Find("a[href$='?tab=following'] span.text-bold").First().Text())
	if following != "" {
		profile.Following = following
	}

	company := strings.TrimSpace(doc.Find("li[itemprop='worksFor'] span").First().Text())
	if company != "" {
		profile.Company = company
	}

	location := strings.TrimSpace(doc.Find("li[itemprop='homeLocation'] span").First().Text())
	if location != "" {
		profile.Location = location
	}

	blog := strings.TrimSpace(doc.Find("li[itemprop='url'] a").First().Text())
	if blog != "" {
		profile.Blog = blog
	}

	parseContributionsDoc(doc, profile)
	if len(profile.ContributionDays) == 0 {
		fetchContributionsFragment(username, profile)
	}

	// Extract pinned repositories (in order)
	pinnedSet := make(map[string]bool)
	doc.Find("ol.d-flex li.pinned-item-list-item").Each(func(i int, s *goquery.Selection) {
		repoName := strings.TrimSpace(s.Find("span.repo").First().Text())
		if repoName != "" && !pinnedSet[repoName] {
			pinnedSet[repoName] = true
			profile.PinnedRepos = append(profile.PinnedRepos, repoName)
		}
	})

	return nil
}

func fetchContributionsFragment(username string, profile *UserProfile) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fragmentURL := fmt.Sprintf("%s/users/%s/contributions", githubWebBaseURL, url.PathEscape(username))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fragmentURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "ghtrend")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := apiHTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return
	}

	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return
	}

	parseContributionsDoc(doc, profile)
}

func parseContributionsDoc(doc *goquery.Document, profile *UserProfile) {
	if profile.TotalContributions == "" {
		doc.Find("h2.f4.text-normal").Each(func(i int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if strings.Contains(strings.ToLower(t), "contributions in the last year") {
				parts := strings.Fields(t)
				if len(parts) > 0 {
					profile.TotalContributions = parts[0]
				}
			}
		})
	}

	doc.Find("td.ContributionCalendar-day, rect.ContributionCalendar-day").Each(func(i int, s *goquery.Selection) {
		levelStr, exists := s.Attr("data-level")
		if exists && levelStr != "" {
			var lvl int
			if _, err := fmt.Sscanf(levelStr, "%d", &lvl); err == nil {
				profile.ContributionDays = append(profile.ContributionDays, lvl)
			}
		}
	})
}

func fetchUserProfileFromAPI(username string, profile *UserProfile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	apiPath := fmt.Sprintf("/users/%s", url.PathEscape(username))
	var data struct {
		Name        string `json:"name"`
		Bio         string `json:"bio"`
		AvatarURL   string `json:"avatar_url"`
		Company     string `json:"company"`
		Location    string `json:"location"`
		Followers   int    `json:"followers"`
		Following   int    `json:"following"`
		PublicRepos int    `json:"public_repos"`
		Blog        string `json:"blog"`
	}

	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", &data); err != nil {
		return err
	}

	if profile.Name == "" && data.Name != "" {
		profile.Name = data.Name
	}
	if profile.Bio == "" && data.Bio != "" {
		profile.Bio = data.Bio
	}
	if profile.AvatarURL == "" && data.AvatarURL != "" {
		profile.AvatarURL = data.AvatarURL
	}
	if profile.Company == "" && data.Company != "" {
		profile.Company = data.Company
	}
	if profile.Location == "" && data.Location != "" {
		profile.Location = data.Location
	}
	if profile.Blog == "" && data.Blog != "" {
		profile.Blog = data.Blog
	}
	if profile.Followers == "" {
		profile.Followers = fmt.Sprintf("%d", data.Followers)
	}
	if profile.Following == "" {
		profile.Following = fmt.Sprintf("%d", data.Following)
	}
	if data.PublicRepos > 0 {
		profile.PublicRepos = data.PublicRepos
	}

	return nil
}
