package update

import (
	"context"
	"net/http"
	"time"
)

type InstallMethod string

const (
	InstallDirect     InstallMethod = "direct"
	InstallHomebrew   InstallMethod = "homebrew"
	InstallScoop      InstallMethod = "scoop"
	InstallChocolatey InstallMethod = "chocolatey"
	InstallAUR        InstallMethod = "aur"
	InstallNix        InstallMethod = "nix"
	InstallWinget     InstallMethod = "winget"
	InstallPackage    InstallMethod = "package"
	InstallGoInstall  InstallMethod = "go-install"
	InstallUnknown    InstallMethod = "unknown"
)

const (
	defaultOwner           = "poizdev"
	defaultRepo            = "zipit"
	defaultCheckInterval   = 24 * time.Hour
	defaultExplicitTimeout = 10 * time.Second
	maxResponseBytes       = 1 << 20 // 1 MB
)

type Release struct {
	Version     string
	TagName     string
	HTMLURL     string
	PublishedAt time.Time
}

type CheckResult struct {
	Current        string
	Latest         *Release
	IsDevBuild     bool
	NewerAvailable bool
}

type Client struct {
	httpClient      *http.Client
	cacheDir        string
	baseURL         string
	nowFunc         func() time.Time
	userAgent       string
	explicitTimeout time.Duration
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		client.httpClient = c
	}
}

func WithCacheDir(dir string) Option {
	return func(client *Client) {
		client.cacheDir = dir
	}
}

func WithNow(f func() time.Time) Option {
	return func(client *Client) {
		client.nowFunc = f
	}
}

func WithUserAgent(ua string) Option {
	return func(client *Client) {
		client.userAgent = ua
	}
}

func WithTimeout(d time.Duration) Option {
	return func(client *Client) {
		client.explicitTimeout = d
	}
}

// WithBaseURL overrides the GitHub API base URL (for testing).
func WithBaseURL(url string) Option {
	return func(client *Client) {
		client.baseURL = url
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient:      &http.Client{Timeout: defaultExplicitTimeout},
		nowFunc:         time.Now,
		explicitTimeout: defaultExplicitTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Check(ctx context.Context, currentVersion string) (*CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.explicitTimeout)
	defer cancel()

	release, err := fetchLatestRelease(ctx, c)
	if err != nil {
		return nil, err
	}

	res := &CheckResult{
		Current:    currentVersion,
		Latest:     release,
		IsDevBuild: IsDevVersion(currentVersion),
	}

	if release != nil && !res.IsDevBuild {
		res.NewerAvailable = IsNewerStable(currentVersion, release.Version)
	}

	// Update cache with whatever we found (or empty state to record timestamp).
	if c.cacheDir != "" {
		var latestVersion, releaseURL string
		var releaseDate time.Time
		if release != nil {
			latestVersion = release.Version
			releaseURL = release.HTMLURL
			releaseDate = release.PublishedAt
		}
		_ = saveCache(c.cacheDir, &CacheState{
			SchemaVersion: 1,
			LastCheck:     c.nowFunc().UTC(),
			LatestStable:  latestVersion,
			ReleaseURL:    releaseURL,
			ReleaseDate:   releaseDate,
		})
	}

	return res, nil
}

func (c *Client) CheckImplicit(_ context.Context, currentVersion string) (*CheckResult, error) {
	if c.cacheDir == "" {
		return nil, nil // No cache, no implicit check
	}

	state, err := loadCache(c.cacheDir)
	if err != nil || state == nil || cacheExpired(state, c.nowFunc(), defaultCheckInterval) {
		return nil, nil
	}

	res := &CheckResult{
		Current:    currentVersion,
		IsDevBuild: IsDevVersion(currentVersion),
	}
	if state.LatestStable != "" {
		res.Latest = &Release{
			Version:     state.LatestStable,
			HTMLURL:     state.ReleaseURL,
			PublishedAt: state.ReleaseDate,
		}
		if !res.IsDevBuild {
			res.NewerAvailable = IsNewerStable(currentVersion, state.LatestStable)
		}
	}
	return res, nil
}
