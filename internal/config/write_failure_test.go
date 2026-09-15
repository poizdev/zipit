package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCleansTemporaryFileWhenPublishFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zipit", "config.toml")
	publishError := errors.New("injected publish failure")
	err := write(path, Empty(), func(_, _ string) error { return publishError })
	if !errors.Is(err, publishError) {
		t.Fatalf("write() error = %v, want %v", err, publishError)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("final config exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".zipit-config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary config files remain: %v", matches)
	}
}
