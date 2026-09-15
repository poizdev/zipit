package archive_test

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ziparchive "github.com/poizdev/zipit/internal/archive"
	"github.com/poizdev/zipit/internal/rules"
)

func TestDryRunAndCreateReturnIdenticalHonestStatistics(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	for name, contents := range map[string]string{
		"src/main.go":                        "12345",
		"README.md":                          "1234567",
		"debug.log":                          "excluded file",
		"node_modules/pkg/deep/generated.js": "must not be visited",
	} {
		path := filepath.Join(source, filepath.FromSlash(name))
		mustMkdir(t, filepath.Dir(path))
		mustWriteFile(t, path, contents)
	}
	matcher, err := rules.Compile([]string{"node_modules/", "*.log"})
	if err != nil {
		t.Fatal(err)
	}

	dryOutput := filepath.Join(parent, "dry.zip")
	dry, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source, Output: dryOutput, Matcher: matcher, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dryOutput); !os.IsNotExist(err) {
		t.Fatalf("dry-run output exists or stat failed: %v", err)
	}
	assertNoArchiveTemps(t, parent)

	realOutput := filepath.Join(parent, "real.zip")
	real, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source, Output: realOutput, Matcher: matcher,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Stats != real.Stats {
		t.Fatalf("dry stats = %+v, real stats = %+v", dry.Stats, real.Stats)
	}
	want := ziparchive.Stats{
		FilesIncluded:       2,
		FilesExcluded:       1,
		DirectoriesExcluded: 1,
		BytesIncluded:       12,
	}
	if real.Stats != want {
		t.Fatalf("stats = %+v, want %+v", real.Stats, want)
	}
	if real.ArchiveSize <= 0 {
		t.Fatalf("archive size = %d", real.ArchiveSize)
	}
}

func TestForceSafelyReplacesExistingArchive(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "new.txt"), "new content")
	mustWriteFile(t, output, "old archive")

	if _, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output}); err == nil {
		t.Fatal("non-force run replaced existing output")
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "old archive" {
		t.Fatalf("non-force output = %q, %v", got, err)
	}
	if _, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Force: true}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("forced output is not a ZIP: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 1 || reader.File[0].Name != "new.txt" {
		t.Fatalf("forced archive entries = %v", reader.File)
	}
}

type cancellingMatcher struct {
	cancel context.CancelFunc
}

type removingMatcher struct {
	target string
}

type steppedCancelContext struct {
	context.Context
	calls    int
	cancelAt int
}

func (c *steppedCancelContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestCancellationDuringFileCopyCleansTempAndPreservesExistingArchive(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "large.bin"), strings.Repeat("x", 128*1024))
	mustWriteFile(t, output, "old archive")
	ctx := &steppedCancelContext{Context: context.Background(), cancelAt: 5}

	_, err := ziparchive.Run(ctx, ziparchive.Request{Source: source, Output: output, Force: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want cancellation during copy", err)
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "old archive" {
		t.Fatalf("cancelled copy output = %q, %v", got, readErr)
	}
	assertNoArchiveTemps(t, parent)
}

func (m removingMatcher) Match(path string, _ bool) bool {
	if path == filepath.Base(m.target) {
		_ = os.Remove(m.target)
	}
	return false
}

func TestGenerationFailureDuringForcePreservesExistingArchive(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	target := filepath.Join(source, "disappears.txt")
	mustMkdir(t, source)
	mustWriteFile(t, target, "content")
	mustWriteFile(t, output, "old archive")

	_, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source, Output: output, Force: true, Matcher: removingMatcher{target: target},
	})
	if err == nil {
		t.Fatal("Run() error = nil after injected generation failure")
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "old archive" {
		t.Fatalf("failed force output = %q, %v", got, readErr)
	}
	assertNoArchiveTemps(t, parent)
}

func (m cancellingMatcher) Match(string, bool) bool {
	m.cancel()
	return false
}

func TestCancellationDuringForcedReplacementPreservesExistingArchive(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "one.txt"), strings.Repeat("x", 1024))
	mustWriteFile(t, filepath.Join(source, "two.txt"), strings.Repeat("y", 1024))
	mustWriteFile(t, output, "old archive")
	ctx, cancel := context.WithCancel(context.Background())

	_, err := ziparchive.Run(ctx, ziparchive.Request{
		Source: source, Output: output, Force: true, Matcher: cancellingMatcher{cancel: cancel},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "old archive" {
		t.Fatalf("cancelled force output = %q, %v", got, readErr)
	}
	assertNoArchiveTemps(t, parent)
}

func TestDryRunIgnoresExistingOutputWithoutMutation(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "a.txt"), "alpha")
	mustWriteFile(t, output, "existing")

	result, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source, Output: output, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FilesIncluded != 1 || result.Stats.BytesIncluded != 5 {
		t.Fatalf("dry-run stats = %+v", result.Stats)
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "existing" {
		t.Fatalf("existing output = %q, %v", got, err)
	}
	assertNoArchiveTemps(t, parent)
}

func TestForcedOutputInsideSourceNeverEntersArchive(t *testing.T) {
	source := t.TempDir()
	output := filepath.Join(source, "archive.zip")
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep")
	mustWriteFile(t, output, "old archive")

	if _, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source, Output: output, Force: true,
	}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if entry.Name == "archive.zip" || strings.HasPrefix(entry.Name, ".zipit-") {
			t.Fatalf("archive contains runtime output %q", entry.Name)
		}
	}
	if len(reader.File) != 1 || reader.File[0].Name != "keep.txt" {
		t.Fatalf("archive entries = %v", reader.File)
	}
}

func assertNoArchiveTemps(t *testing.T, directory string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(directory, ".zipit-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary archives remain: %v", matches)
	}
}
