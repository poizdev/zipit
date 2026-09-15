package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/rules"
)

func TestLoadIgnoreFileMissingIsEmpty(t *testing.T) {
	rulesFromFile, err := rules.LoadIgnoreFile(t.TempDir())
	if err != nil {
		t.Fatalf("LoadIgnoreFile() error = %v", err)
	}
	if len(rulesFromFile) != 0 {
		t.Fatalf("LoadIgnoreFile() = %v, want no rules", rulesFromFile)
	}
}

func TestLoadIgnoreFileParsesRulesCommentsAndEmptyLines(t *testing.T) {
	source := t.TempDir()
	contents := "  # dependencies\n\n  node_modules/  \n\t# logs\n *.log\t\n**/.cache/**\n"
	if err := os.WriteFile(filepath.Join(source, ".zipitignore"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := rules.LoadIgnoreFile(source)
	if err != nil {
		t.Fatalf("LoadIgnoreFile() error = %v", err)
	}
	wantPatterns := []string{"node_modules/", "*.log", "**/.cache/**"}
	wantLines := []int{3, 5, 6}
	if len(got) != len(wantPatterns) {
		t.Fatalf("LoadIgnoreFile() = %v, want %d rules", got, len(wantPatterns))
	}
	for i := range wantPatterns {
		if got[i].Pattern != wantPatterns[i] || got[i].Line != wantLines[i] {
			t.Errorf("rule %d = %+v, want pattern %q at line %d", i, got[i], wantPatterns[i], wantLines[i])
		}
		if filepath.Base(got[i].Source) != ".zipitignore" {
			t.Errorf("rule %d source = %q, want .zipitignore", i, got[i].Source)
		}
	}
}

func TestCompileRulesReportsIgnoreFileLine(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, ".zipitignore"), []byte("# valid\n*.log\n[\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := rules.LoadIgnoreFile(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rules.CompileRules(loaded)
	if err == nil {
		t.Fatal("CompileRules() error = nil, want invalid-rule error")
	}
	if !strings.Contains(err.Error(), ".zipitignore:3") {
		t.Fatalf("CompileRules() error = %q, want source and line", err)
	}
}
