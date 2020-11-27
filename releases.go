package eco

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

const releasesURL = "https://api.github.com/repos/decred/decred-binaries/releases"

type githubRelease struct {
	Name       string         `json:"name"`
	Prerelease bool           `json:"prerelease"`
	Published  time.Time      `json:"published_at"`
	Assets     []*githubAsset `json:"assets"`
}

type githubAsset struct {
	URL                string     `json:"url"`
	ID                 uint32     `json:"id"`
	Name               string     `json:"name"`
	Uploader           githubUser `json:"uploader"`
	ContentType        string     `json:"content_type"`
	Size               uint32     `json:"size"`
	DownloadCount      uint32     `json:"download_count"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	BrowserDownloadURL string     `json:"browser_download_url"`
	Author             githubUser `json:"author"`
}

type githubUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func fetchReleases() ([]*githubRelease, error) {
	resp, err := http.Get(releasesURL)
	if err != nil {
		return nil, fmt.Errorf("Error fetching releases: %w", err)
	}

	defer resp.Body.Close()
	var releases []*githubRelease
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		return nil, fmt.Errorf("JSON decode error: %w", err)
	}
	//  Should already be sorted newest first, but just make sure.
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Published.After(releases[j].Published)
	})
	return releases, nil
}
