package archive

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateCleansTemporaryFileWhenPublicationFails(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	publishError := errors.New("injected publication failure")
	err := create(Request{Source: source, Output: output}, func(_, _ string) error {
		return publishError
	})
	if !errors.Is(err, publishError) {
		t.Fatalf("create() error = %v, want %v", err, publishError)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("final output exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestNoReplacePublicationRaceReturnsActionableError(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run(context.Background(), Request{Source: source, Output: output}, func(_, final string) error {
		if writeErr := os.WriteFile(final, []byte("race winner"), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
		return fs.ErrExist
	})
	if err == nil || !strings.Contains(err.Error(), "use --force") {
		t.Fatalf("run() error = %v, want actionable existing-output error", err)
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "race winner" {
		t.Fatalf("race output = %q, %v", got, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, %v", matches, globErr)
	}
}

func TestCancellationBeforeForcedPublishPreservesExistingArchive(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("old archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	publishCalled := false

	_, err := runWithBeforePublish(ctx, Request{Source: source, Output: output, Force: true}, func(_, _ string) error {
		publishCalled = true
		return nil
	}, cancel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runWithBeforePublish() error = %v, want cancellation", err)
	}
	if publishCalled {
		t.Fatal("publisher called after cancellation")
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "old archive" {
		t.Fatalf("cancelled publish output = %q, %v", got, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, %v", matches, globErr)
	}
}
