package archive_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ziparchive "github.com/poizdev/zipit/internal/archive"
	"github.com/poizdev/zipit/internal/rules"
)

func generateDevCoWorkspace(tb testing.TB, dependencyFiles int) string {
	tb.Helper()
	root := tb.TempDir()
	projects := []string{"auth-layer", "devcast", "devtv", "devtake"}
	for _, project := range projects {
		for _, name := range []string{"src/main.go", "README.md", ".git/config", "cache/state", "recordings/clip.dat", "generated/client.go"} {
			path := filepath.Join(root, project, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(project+name), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
		for i := 0; i < dependencyFiles; i++ {
			path := filepath.Join(root, project, "node_modules", fmt.Sprintf("pkg-%04d", i/100), fmt.Sprintf("file-%05d.js", i))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("generated dependency"), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return root
}

func benchmarkTraversal(b *testing.B, prune bool) {
	source := generateDevCoWorkspace(b, 2500)
	var matcher ziparchive.Matcher
	if prune {
		compiled, err := rules.Compile([]string{"node_modules/"})
		if err != nil {
			b.Fatal(err)
		}
		matcher = compiled
	}
	output := filepath.Join(b.TempDir(), "unused.zip")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Matcher: matcher, DryRun: true})
		if err != nil {
			b.Fatal(err)
		}
		if prune && result.Stats.DirectoriesExcluded != 4 {
			b.Fatalf("pruned directories = %d, want 4", result.Stats.DirectoriesExcluded)
		}
	}
}

func BenchmarkTraversalWithoutExclusion(b *testing.B) { benchmarkTraversal(b, false) }

func BenchmarkTraversalWithDirectoryPruning(b *testing.B) { benchmarkTraversal(b, true) }
