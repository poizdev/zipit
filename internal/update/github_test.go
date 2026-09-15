package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchLatestRelease_ValidNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/poizdev/zipit/releases/latest" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.2.0",
			HTMLURL:     "https://github.com/poizdev/zipit/releases/tag/v0.2.0",
			PublishedAt: "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.NewerAvailable {
		t.Fatal("expected NewerAvailable = true")
	}
	if result.Latest.Version != "v0.2.0" {
		t.Fatalf("Latest.Version = %q, want v0.2.0", result.Latest.Version)
	}
	if result.Latest.HTMLURL != "https://github.com/poizdev/zipit/releases/tag/v0.2.0" {
		t.Fatalf("Latest.HTMLURL = %q", result.Latest.HTMLURL)
	}
}

func TestFetchLatestRelease_SameVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.1.0",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.NewerAvailable {
		t.Fatal("expected NewerAvailable = false for same version")
	}
}

func TestFetchLatestRelease_OlderVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.1.0",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.NewerAvailable {
		t.Fatal("expected NewerAvailable = false for older remote")
	}
}

func TestFetchLatestRelease_DraftIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.3.0",
			Draft:   true,
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Latest != nil {
		t.Fatalf("expected nil Latest for draft release, got %+v", result.Latest)
	}
}

func TestFetchLatestRelease_PrereleaseIgnored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:    "v0.3.0-beta.1",
			Prerelease: true,
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Latest != nil {
		t.Fatalf("expected nil Latest for prerelease, got %+v", result.Latest)
	}
}

func TestFetchLatestRelease_PrereleaseTag(t *testing.T) {
	// Prerelease in tag name but not flagged as prerelease on GitHub.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.3.0-rc.1",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Latest != nil {
		t.Fatalf("expected nil Latest for prerelease tag, got %+v", result.Latest)
	}
}

func TestFetchLatestRelease_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid json"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.Check(context.Background(), "v0.1.0")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse release") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchLatestRelease_MalformedTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "not-semver",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.Check(context.Background(), "v0.1.0")
	if err == nil {
		t.Fatal("expected error for malformed tag")
	}
	if !strings.Contains(err.Error(), "invalid release tag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchLatestRelease_Non2xxResponse(t *testing.T) {
	for _, status := range []int{500, 502, 503} {
		t.Run(fmt.Sprintf("HTTP_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			client := newTestClient(t, srv)
			_, err := client.Check(context.Background(), "v0.1.0")
			if err == nil {
				t.Fatalf("expected error for HTTP %d", status)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", status)) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFetchLatestRelease_RateLimited(t *testing.T) {
	for _, status := range []int{403, 429} {
		t.Run(fmt.Sprintf("HTTP_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			client := newTestClient(t, srv)
			_, err := client.Check(context.Background(), "v0.1.0")
			if err == nil {
				t.Fatalf("expected error for rate limit HTTP %d", status)
			}
			if !strings.Contains(err.Error(), "rate limit") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFetchLatestRelease_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v (want nil for no releases)", err)
	}
	if result.Latest != nil {
		t.Fatalf("expected nil Latest for 404, got %+v", result.Latest)
	}
}

func TestFetchLatestRelease_OversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write more than maxResponseBytes. LimitReader will truncate, causing JSON parse failure.
		huge := strings.Repeat("x", maxResponseBytes+1024)
		w.Write([]byte(`{"tag_name":"v0.1.0","html_url":"` + huge + `"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.Check(context.Background(), "v0.1.0")
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
}

func TestFetchLatestRelease_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response.
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.Check(ctx, "v0.1.0")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchLatestRelease_DevBuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.1.0",
			HTMLURL:     "https://github.com/poizdev/zipit/releases/tag/v0.1.0",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	result, err := client.Check(context.Background(), "dev")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.IsDevBuild {
		t.Fatal("expected IsDevBuild = true")
	}
	if result.NewerAvailable {
		t.Fatal("expected NewerAvailable = false for dev build")
	}
	if result.Latest == nil || result.Latest.Version != "v0.1.0" {
		t.Fatalf("expected Latest.Version = v0.1.0, got %v", result.Latest)
	}
}

func TestFetchLatestRelease_UserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.1.0",
			PublishedAt: "2026-01-01T10:00:00Z",
		})
	}))
	defer srv.Close()

	client := newTestClientWithUA(t, srv, "zipit/test")
	_, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if gotUA != "zipit/test" {
		t.Fatalf("User-Agent = %q, want %q", gotUA, "zipit/test")
	}
}

// newTestClient creates a Client that talks to the test server,
// rewriting the GitHub API URL to point at the test server.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return newTestClientWithUA(t, srv, "")
}

func newTestClientWithUA(t *testing.T, srv *httptest.Server, ua string) *Client {
	t.Helper()
	return NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
		WithUserAgent(ua),
		WithCacheDir(t.TempDir()),
	)
}
