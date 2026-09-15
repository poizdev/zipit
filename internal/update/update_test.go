package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckImplicit_FirstCheckDoesNotUseNetwork(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.2.0",
			HTMLURL:     "https://github.com/poizdev/zipit/releases/tag/v0.2.0",
			PublishedAt: "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	client := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return now }),
	)

	result, err := client.CheckImplicit(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("CheckImplicit() error = %v", err)
	}
	if result != nil {
		t.Fatalf("expected no cached result, got %+v", result)
	}
	if requests.Load() != 0 {
		t.Fatalf("expected no HTTP request, got %d", requests.Load())
	}
}

func TestCheckImplicit_SecondCallInsideIntervalUsesCache(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.2.0",
			PublishedAt: "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	client := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return now }),
	)

	// Explicit discovery populates the cache.
	_, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}

	// Second check 1 hour later: should use cache.
	later := now.Add(1 * time.Hour)
	client2 := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return later }),
	)

	result, err := client2.CheckImplicit(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.NewerAvailable {
		t.Fatal("expected NewerAvailable = true from cache")
	}
	if requests.Load() != 1 {
		t.Fatalf("expected 1 total HTTP request, got %d", requests.Load())
	}
}

func TestCheckImplicit_ExpiredCacheNeverUsesNetwork(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := saveCache(cacheDir, &CacheState{SchemaVersion: 1, LastCheck: now.Add(-25 * time.Hour), LatestStable: "v0.2.0"}); err != nil {
		t.Fatal(err)
	}
	client := NewClient(
		WithHTTPClient(&http.Client{Transport: panicTransport{}}),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return now }),
	)

	result, err := client.CheckImplicit(context.Background(), "v0.1.0")
	if err != nil || result != nil {
		t.Fatalf("CheckImplicit() = %+v, %v; want expired cache ignored", result, err)
	}
}

type panicTransport struct{}

func (panicTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("implicit update check attempted network discovery")
}

func TestCheck_ExplicitBypassesCache(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.2.0",
			PublishedAt: "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	client := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return now }),
		WithTimeout(5*time.Second),
	)

	// First explicit check caches.
	_, _ = client.Check(context.Background(), "v0.1.0")

	// A second explicit check always fetches regardless of cache.
	_, err := client.Check(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected 2 HTTP requests from explicit checks, got %d", requests.Load())
	}
}

func TestCheckImplicit_NoCacheDir(t *testing.T) {
	client := NewClient()
	result, err := client.CheckImplicit(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("expected nil result without cache dir, got %+v", result)
	}
}

func TestCheckImplicit_FreshCacheDoesNotRecheckNetwork(t *testing.T) {
	// An explicit successful check populates cache.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(githubRelease{
			TagName:     "v0.2.0",
			PublishedAt: "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	client := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return now }),
	)

	// Explicit discovery succeeds and caches.
	result, err := client.Check(context.Background(), "v0.1.0")
	if err != nil || result == nil {
		t.Fatalf("explicit check: err=%v, result=%v", err, result)
	}

	// Second call within interval: should return from cache.
	later := now.Add(1 * time.Hour)
	client2 := NewClient(
		WithHTTPClient(srv.Client()),
		WithBaseURL(srv.URL),
		WithCacheDir(cacheDir),
		WithNow(func() time.Time { return later }),
	)
	result2, err := client2.CheckImplicit(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if result2 == nil || !result2.NewerAvailable {
		t.Fatal("expected cached result to report newer available")
	}
	if callCount != 1 {
		t.Fatalf("implicit check made a network request; total requests = %d", callCount)
	}
}
