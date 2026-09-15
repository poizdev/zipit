package archive

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/rules"
)

type recordingMatcher struct {
	matcher *rules.Matcher
	seen    []string
}

func (m *recordingMatcher) Match(path string, isDir bool) bool {
	m.seen = append(m.seen, path)
	return m.matcher.Match(path, isDir)
}

func TestCreateAppliesExclusionsAndPrunesDirectories(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	files := map[string]string{
		"src/main.go":                        "package main",
		"node_modules/pkg/deep/generated.js": "generated",
		"nested/node_modules/pkg/index.js":   "generated",
		"nested/app.js":                      "app",
		"logs/debug.log":                     "debug",
		"README.md":                          "readme",
	}
	for path, content := range files {
		fullPath := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 1000; i++ {
		for _, excludedRoot := range []string{"node_modules", "nested/node_modules"} {
			fullPath := filepath.Join(source, filepath.FromSlash(excludedRoot), "generated", fmt.Sprintf("file-%04d.js", i))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fullPath, []byte("generated"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	compiled, err := rules.Compile([]string{"node_modules/", "*.log"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingMatcher{matcher: compiled}
	if err := Create(Request{Source: source, Output: output, Matcher: recorder}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	wantEntries := map[string]bool{
		"src/":          true,
		"src/main.go":   true,
		"nested/":       true,
		"nested/app.js": true,
		"logs/":         true,
		"README.md":     true,
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	gotEntries := make(map[string]bool)
	for _, entry := range reader.File {
		gotEntries[entry.Name] = true
	}
	if len(gotEntries) != len(wantEntries) {
		t.Fatalf("archive entries = %v, want %v", gotEntries, wantEntries)
	}
	for name := range wantEntries {
		if !gotEntries[name] {
			t.Errorf("archive missing %q", name)
		}
	}

	for _, visited := range recorder.seen {
		if strings.HasPrefix(visited, "node_modules/") || strings.HasPrefix(visited, "nested/node_modules/") {
			t.Fatalf("excluded directory child %q was visited; subtree was not pruned", visited)
		}
	}
}

func TestIgnoreFileDirectoryRulePrunesBeforeVisitingChildren(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	if err := os.MkdirAll(filepath.Join(source, "node_modules", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".zipitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		path := filepath.Join(source, "node_modules", "deep", fmt.Sprintf("file-%04d.js", i))
		if err := os.WriteFile(path, []byte("generated"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := rules.LoadIgnoreFile(source)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := rules.CompileRules(loaded)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingMatcher{matcher: compiled}
	if err := Create(Request{Source: source, Output: output, Matcher: recorder}); err != nil {
		t.Fatal(err)
	}
	for _, visited := range recorder.seen {
		if strings.HasPrefix(visited, "node_modules/") {
			t.Fatalf("ignore-file excluded child %q was visited", visited)
		}
	}
}
