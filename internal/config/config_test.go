package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/config"
)

func TestPathUsesPlatformConfigDirectory(t *testing.T) {
	configHome := isolateConfigHome(t)
	got, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configHome, "zipit", "config.toml")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingReturnsEmptyWithoutCreatingAnything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "zipit", "config.toml")
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != 1 || len(got.Ignore.Patterns) != 0 {
		t.Fatalf("Load() = %+v, want schema-1 empty config", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("read created config directory or stat failed: %v", err)
	}
}

func TestLoadValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "schema = 1\n\n[ignore]\npatterns = [\"node_modules/\", \"*.log\"]\n")
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "node_modules/,*.log" {
		t.Fatalf("patterns = %v", got.Ignore.Patterns)
	}
}

func TestLoadRejectsMalformedAndUnsupportedSchemas(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
		want     string
	}{
		{name: "malformed", contents: "schema = [", want: "parse"},
		{name: "missing schema", contents: "[ignore]\npatterns = []\n", want: "schema"},
		{name: "unsupported schema", contents: "schema = 2\n", want: "unsupported schema 2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, test.contents)
			_, err := config.Load(path)
			if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want path and %q", err, test.want)
			}
		})
	}
}

func TestWriteCreatesReplacesAndCleansTemporaryFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "zipit", "config.toml")
	first := config.Config{Schema: 1}
	first.Ignore.Patterns = []string{"node_modules/"}
	if err := config.Write(path, first); err != nil {
		t.Fatal(err)
	}
	second := config.Config{Schema: 1}
	second.Ignore.Patterns = []string{"*.log"}
	if err := config.Write(path, second); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "*.log" {
		t.Fatalf("reloaded patterns = %v", got.Ignore.Patterns)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".zipit-config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary config files remain: %v", matches)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("config permissions = %o, want user-only", info.Mode().Perm())
		}
	}
}

func TestCreateDoesNotReplaceExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zipit", "config.toml")
	existing := config.Empty()
	existing.Ignore.Patterns = []string{"keep/"}
	if err := config.Write(path, existing); err != nil {
		t.Fatal(err)
	}
	replacement := config.Empty()
	replacement.Ignore.Patterns = []string{"replace/"}
	if err := config.Create(path, replacement); err == nil {
		t.Fatal("Create() replaced an existing config")
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "keep/" {
		t.Fatalf("patterns = %v, want existing config preserved", got.Ignore.Patterns)
	}
}

func isolateConfigHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", root)
		return root
	case "darwin":
		t.Setenv("HOME", root)
		return filepath.Join(root, "Library", "Application Support")
	default:
		t.Setenv("XDG_CONFIG_HOME", root)
		return root
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
