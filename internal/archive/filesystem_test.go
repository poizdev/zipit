//go:build !windows

package archive_test

import (
	"archive/zip"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ziparchive "github.com/poizdev/zipit/internal/archive"
)

func TestSymlinksArePreservedWithoutFollowingTargets(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	outside := filepath.Join(parent, "outside-secret.txt")
	mustMkdir(t, filepath.Join(source, "actual"))
	mustWriteFile(t, filepath.Join(source, "actual", "inside.txt"), "inside")
	mustWriteFile(t, outside, "outside secret")
	links := map[string]string{
		"link-file":   "actual/inside.txt",
		"link-dir":    "actual",
		"broken-link": "does-not-exist",
		"escape-link": outside,
		"cycle":       "cycle",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(source, name)); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
	}
	output := filepath.Join(parent, "archive.zip")
	result, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.SymlinksIncluded != int64(len(links)) {
		t.Fatalf("stats = %+v", result.Stats)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	seen := map[string]string{}
	allNames := map[string]bool{}
	for _, entry := range reader.File {
		allNames[entry.Name] = true
		if _, ok := links[entry.Name]; !ok {
			continue
		}
		if entry.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not marked as a symlink", entry.Name)
		}
		stream, openErr := entry.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		data, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		seen[entry.Name] = string(data)
	}
	for name, target := range links {
		if seen[name] != target {
			t.Errorf("symlink %s content = %q, want %q", name, seen[name], target)
		}
	}
	for _, unexpected := range []string{"link-dir/", "link-dir/inside.txt", "outside-secret.txt", "escape-link/outside-secret.txt"} {
		if allNames[unexpected] {
			t.Fatalf("symlink target entry %q leaked into archive", unexpected)
		}
	}
}

func TestFIFOAndSocketAreSkippedWithoutBlocking(t *testing.T) {
	source := t.TempDir()
	if err := makeFIFO(filepath.Join(source, "pipe")); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	listener, err := net.Listen("unix", filepath.Join(source, "socket"))
	if err != nil {
		t.Skipf("Unix socket unavailable: %v", err)
	}
	defer listener.Close()
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep")
	output := filepath.Join(t.TempDir(), "archive.zip")
	result, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.SpecialFilesSkipped != 2 {
		t.Fatalf("stats = %+v", result.Stats)
	}
}

func TestDryRunParityIncludesSymlinkAndSpecialFileSemantics(t *testing.T) {
	source := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep")
	if err := os.Symlink("keep.txt", filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := makeFIFO(filepath.Join(source, "pipe")); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	parent := t.TempDir()
	dry, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: filepath.Join(parent, "dry.zip"), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	real, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: filepath.Join(parent, "real.zip")})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Stats != real.Stats {
		t.Fatalf("dry stats = %+v, real stats = %+v", dry.Stats, real.Stats)
	}
	if dry.Stats.SymlinksIncluded != 1 || dry.Stats.SpecialFilesSkipped != 1 {
		t.Fatalf("stats = %+v", dry.Stats)
	}
}

func TestSymlinkSourceResolvesToDirectory(t *testing.T) {
	parent := t.TempDir()
	realSource := filepath.Join(parent, "real")
	mustMkdir(t, realSource)
	mustWriteFile(t, filepath.Join(realSource, "file.txt"), "content")
	linkedSource := filepath.Join(parent, "linked")
	if err := os.Symlink(realSource, linkedSource); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	request, err := ziparchive.ResolveRequest(linkedSource, filepath.Join(parent, "archive.zip"))
	if err != nil {
		t.Fatal(err)
	}
	resolvedInfo, err := os.Stat(request.Source)
	if err != nil {
		t.Fatal(err)
	}
	realInfo, err := os.Stat(realSource)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(resolvedInfo, realInfo) {
		t.Fatalf("resolved source = %q, want same filesystem directory as %q", request.Source, realSource)
	}
	defaultRequest, err := ziparchive.ResolveRequest(linkedSource, "")
	if err != nil {
		t.Fatal(err)
	}
	if defaultRequest.Output != defaultRequest.Source+".zip" {
		t.Fatalf("default output = %q, want %q", defaultRequest.Output, defaultRequest.Source+".zip")
	}
}

func TestOutputInsideSourceThroughParentAliasExcludesOnlyRuntimeArtifacts(t *testing.T) {
	parent := t.TempDir()
	realParent := filepath.Join(parent, "real")
	source := filepath.Join(realParent, "workspace")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep")
	mustWriteFile(t, filepath.Join(source, ".zipit-user.tmp"), "user data")

	aliasParent := filepath.Join(parent, "alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	output := filepath.Join(aliasParent, "workspace", "archive.zip")
	mustWriteFile(t, output, "old archive")

	if _, err := ziparchive.Run(context.Background(), ziparchive.Request{
		Source: source,
		Output: output,
		Force:  true,
	}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	seen := make(map[string]bool)
	for _, entry := range reader.File {
		seen[entry.Name] = true
	}
	if seen["archive.zip"] {
		t.Fatal("archive contains its final output")
	}
	for name := range seen {
		if strings.HasPrefix(name, ".zipit-") && name != ".zipit-user.tmp" {
			t.Fatalf("archive contains runtime staging file %q", name)
		}
	}
	for _, name := range []string{"keep.txt", ".zipit-user.tmp"} {
		if !seen[name] {
			t.Fatalf("archive is missing user file %q; entries = %v", name, seen)
		}
	}
}

func TestUnreadableRegularFileFailsWithoutReplacingExistingArchive(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	source := t.TempDir()
	unreadable := filepath.Join(source, "unreadable.txt")
	mustWriteFile(t, unreadable, "secret")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Skipf("cannot create unreadable file: %v", err)
	}
	output := filepath.Join(t.TempDir(), "archive.zip")
	mustWriteFile(t, output, "old archive")
	_, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Force: true})
	if err == nil || !strings.Contains(err.Error(), unreadable) {
		t.Fatalf("Run() error = %v, want unreadable path", err)
	}
	if got, readErr := os.ReadFile(output); readErr != nil || string(got) != "old archive" {
		t.Fatalf("existing output = %q, %v", got, readErr)
	}
}
