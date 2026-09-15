package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/poizdev/zipit/internal/buildinfo"
)

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

func fetchLatestRelease(ctx context.Context, c *Client) (*Release, error) {
	base := c.baseURL
	if base == "" {
		base = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", base, defaultOwner, defaultRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	ua := c.userAgent
	if ua == "" {
		ua = "zipit/" + buildinfo.Version
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases exist yet.
		return nil, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("GitHub API rate limit exceeded (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var gh githubRelease
	if err := json.Unmarshal(body, &gh); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	// Validate even though /releases/latest should already filter these.
	if gh.Draft || gh.Prerelease {
		return nil, nil
	}

	version, err := ParseVersion(gh.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid release tag %q: %w", gh.TagName, err)
	}

	// Reject prereleases at the version level too.
	if IsPrerelease(version) {
		return nil, nil
	}

	var publishedAt time.Time
	if gh.PublishedAt != "" {
		publishedAt, _ = time.Parse(time.RFC3339, gh.PublishedAt)
	}

	return &Release{
		Version:     version,
		TagName:     gh.TagName,
		HTMLURL:     gh.HTMLURL,
		PublishedAt: publishedAt.UTC(),
	}, nil
}
