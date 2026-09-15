package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadCache_Missing(t *testing.T) {
	dir := t.TempDir()
	state, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}
	if state != nil {
		t.Fatalf("expected nil state for missing cache, got %+v", state)
	}
}

func TestSaveAndLoadCache_Valid(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	original := &CacheState{
		SchemaVersion: 1,
		LastCheck:     now,
		LatestStable:  "v0.2.0",
		ReleaseURL:    "https://github.com/poizdev/zipit/releases/tag/v0.2.0",
		ReleaseDate:   now.Add(-24 * time.Hour),
	}

	if err := saveCache(dir, original); err != nil {
		t.Fatalf("saveCache() error = %v", err)
	}

	loaded, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("loadCache() returned nil")
	}
	if loaded.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", loaded.SchemaVersion)
	}
	if loaded.LatestStable != "v0.2.0" {
		t.Fatalf("LatestStable = %q, want v0.2.0", loaded.LatestStable)
	}
	if !loaded.LastCheck.Equal(now) {
		t.Fatalf("LastCheck = %v, want %v", loaded.LastCheck, now)
	}
}

func TestLoadCache_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cacheFileName)
	if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache() error = %v (expected nil for corrupt)", err)
	}
	if state != nil {
		t.Fatalf("expected nil state for corrupt cache, got %+v", state)
	}
}

func TestLoadCache_UnsupportedSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cacheFileName)
	data, _ := json.Marshal(CacheState{SchemaVersion: 99})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := loadCache(dir)
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}
	if state != nil {
		t.Fatalf("expected nil state for unsupported schema, got %+v", state)
	}
}

func TestCacheExpired(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	interval := 24 * time.Hour

	tests := []struct {
		name      string
		lastCheck time.Time
		want      bool
	}{
		{name: "fresh", lastCheck: now.Add(-1 * time.Hour), want: false},
		{name: "exactly expired", lastCheck: now.Add(-24 * time.Hour), want: true},
		{name: "long expired", lastCheck: now.Add(-48 * time.Hour), want: true},
		{name: "just under", lastCheck: now.Add(-23*time.Hour - 59*time.Minute), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &CacheState{LastCheck: tt.lastCheck}
			got := cacheExpired(state, now, interval)
			if got != tt.want {
				t.Fatalf("cacheExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveCache_AtomicReplacement(t *testing.T) {
	dir := t.TempDir()

	// Write initial cache.
	initial := &CacheState{
		SchemaVersion: 1,
		LastCheck:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LatestStable:  "v0.1.0",
	}
	if err := saveCache(dir, initial); err != nil {
		t.Fatal(err)
	}

	// Update cache.
	updated := &CacheState{
		SchemaVersion: 1,
		LastCheck:     time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		LatestStable:  "v0.2.0",
	}
	if err := saveCache(dir, updated); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LatestStable != "v0.2.0" {
		t.Fatalf("LatestStable = %q, want v0.2.0", loaded.LatestStable)
	}

	// No temp files should remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != cacheFileName {
			t.Fatalf("unexpected file remaining: %s", e.Name())
		}
	}
}

func TestSaveCache_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "cache")

	state := &CacheState{
		SchemaVersion: 1,
		LatestStable:  "v0.1.0",
	}
	if err := saveCache(dir, state); err != nil {
		t.Fatalf("saveCache() error = %v", err)
	}

	loaded, err := loadCache(dir)
	if err != nil || loaded == nil {
		t.Fatalf("loadCache() = %v, %v", loaded, err)
	}
}

func TestSaveCache_WriteFailure(t *testing.T) {
	// Use a path that cannot be created.
	dir := filepath.Join(t.TempDir(), "nonexistent")
	if err := os.WriteFile(dir, []byte("file not dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "cache")

	state := &CacheState{SchemaVersion: 1}
	err := saveCache(nested, state)
	if err == nil {
		t.Fatal("expected error for non-writable path")
	}
}

func TestDefaultCacheDir(t *testing.T) {
	dir, err := DefaultCacheDir()
	if err != nil {
		t.Fatalf("DefaultCacheDir() error = %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %q", dir)
	}
	if filepath.Base(dir) != "zipit" {
		t.Fatalf("expected dir ending in 'zipit', got %q", dir)
	}
}
