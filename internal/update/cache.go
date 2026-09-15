package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheState holds persisted update discovery state.
type CacheState struct {
	SchemaVersion int       `json:"schema_version"`
	LastCheck     time.Time `json:"last_check"`
	LatestStable  string    `json:"latest_stable,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
	ReleaseDate   time.Time `json:"release_date,omitempty"`
}

const (
	cacheFileName    = "update-state.json"
	currentSchemaVer = 1
)

func loadCache(dir string) (*CacheState, error) {
	data, err := os.ReadFile(filepath.Join(dir, cacheFileName))
	if err != nil {
		return nil, nil // Missing file is not an error.
	}

	var state CacheState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, nil // Corrupt cache: silently reset.
	}
	if state.SchemaVersion != currentSchemaVer {
		return nil, nil // Unsupported schema: silently reset.
	}
	return &state, nil
}

func saveCache(dir string, state *CacheState) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, "update-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp cache file: %w", err)
	}

	target := filepath.Join(dir, cacheFileName)
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("publish cache file: %w", err)
	}
	return nil
}

func cacheExpired(state *CacheState, now time.Time, interval time.Duration) bool {
	return now.UTC().Sub(state.LastCheck.UTC()) >= interval
}

// DefaultCacheDir returns the default cache directory for update state.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache directory: %w", err)
	}
	return filepath.Join(base, "zipit"), nil
}
