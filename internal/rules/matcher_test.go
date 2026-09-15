package rules_test

import (
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/rules"
)

func TestMatcherSemantics(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		isDir   bool
		want    bool
	}{
		{name: "basename at root", pattern: ".DS_Store", path: ".DS_Store", want: true},
		{name: "basename nested", pattern: ".DS_Store", path: "foo/bar/.DS_Store", want: true},
		{name: "basename mismatch", pattern: ".DS_Store", path: "foo/.DS_Store.bak", want: false},
		{name: "basename glob at root", pattern: "*.log", path: "app.log", want: true},
		{name: "basename glob nested", pattern: "*.log", path: "a/b/debug.log", want: true},
		{name: "file rule does not match directory", pattern: "*.log", path: "logs.log", isDir: true, want: false},
		{name: "directory at root", pattern: "node_modules/", path: "node_modules", isDir: true, want: true},
		{name: "directory nested", pattern: "node_modules/", path: "foo/node_modules", isDir: true, want: true},
		{name: "git directory nested", pattern: ".git/", path: "foo/.git", isDir: true, want: true},
		{name: "directory rule does not match file", pattern: "node_modules/", path: "node_modules", want: false},
		{name: "similarly named directory mismatch", pattern: "node_modules/", path: "node_modules-old", isDir: true, want: false},
		{name: "root relative directory", pattern: "devtv/recordings/", path: "devtv/recordings", isDir: true, want: true},
		{name: "leading slash root relative directory", pattern: "/devtv/recordings/", path: "devtv/recordings", isDir: true, want: true},
		{name: "root relative does not float", pattern: "devtv/recordings/", path: "foo/devtv/recordings", isDir: true, want: false},
		{name: "recursive cache root", pattern: "**/.cache/**", path: ".cache", isDir: true, want: true},
		{name: "recursive cache nested", pattern: "**/.cache/**", path: "foo/.cache", isDir: true, want: true},
		{name: "recursive cache child", pattern: "**/.cache/**", path: "foo/.cache/data.bin", want: true},
		{name: "recursive asset glob direct", pattern: "assets/**/*.map", path: "assets/app.map", want: true},
		{name: "recursive asset glob nested", pattern: "assets/**/*.map", path: "assets/js/app.map", want: true},
		{name: "recursive file glob does not match directory", pattern: "assets/**/*.map", path: "assets/js/app.map", isDir: true, want: false},
		{name: "recursive asset glob anchored", pattern: "assets/**/*.map", path: "other/assets/app.map", want: false},
		{name: "root relative file rule does not match directory", pattern: "foo/bar", path: "foo/bar", isDir: true, want: false},
		{name: "leading dot slash rule", pattern: "./foo", path: "foo", want: true},
		{name: "leading dot slash input", pattern: "foo", path: "./foo", want: true},
		{name: "windows rule separators", pattern: `foo\bar`, path: "foo/bar", want: true},
		{name: "windows input separators", pattern: "foo/bar", path: `foo\bar`, want: true},
		{name: "candidate cannot clean through parent", pattern: "secret", path: "foo/../secret", want: false},
		{name: "case sensitive", pattern: "*.log", path: "APP.LOG", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher, err := rules.Compile([]string{test.pattern})
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got := matcher.Match(test.path, test.isDir); got != test.want {
				t.Fatalf("Match(%q, %v) = %v, want %v", test.path, test.isDir, got, test.want)
			}
		})
	}
}

func TestCompileComposesRules(t *testing.T) {
	matcher, err := rules.Compile([]string{"node_modules/", "*.log"})
	if err != nil {
		t.Fatal(err)
	}
	if !matcher.Match("nested/node_modules", true) {
		t.Fatal("directory rule did not compose")
	}
	if !matcher.Match("logs/debug.log", false) {
		t.Fatal("basename rule did not compose")
	}
	if matcher.Match("src/main.go", false) {
		t.Fatal("unmatched file was excluded")
	}
}

func TestCompileRejectsInvalidRules(t *testing.T) {
	for _, pattern := range []string{"[", "../secret", "foo/../secret", "!important.log", ""} {
		t.Run(pattern, func(t *testing.T) {
			_, err := rules.Compile([]string{pattern})
			if err == nil {
				t.Fatalf("Compile(%q) error = nil", pattern)
			}
			if !strings.Contains(err.Error(), "invalid exclusion rule") {
				t.Fatalf("Compile(%q) error = %q, want useful rule context", pattern, err)
			}
		})
	}
}

func TestCanonical(t *testing.T) {
	for _, test := range []struct{ raw, want string }{
		{raw: `.\cache\`, want: "cache/"},
		{raw: "./node_modules/", want: "node_modules/"},
		{raw: "*.log", want: "*.log"},
	} {
		got, err := rules.Canonical(test.raw)
		if err != nil {
			t.Fatalf("Canonical(%q) error = %v", test.raw, err)
		}
		if got != test.want {
			t.Fatalf("Canonical(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestCanonicalRejectsRootAfterPrefixNormalization(t *testing.T) {
	if canonical, err := rules.Canonical("/./"); err == nil {
		t.Fatalf("Canonical(/./) = %q, want controlled invalid-rule error", canonical)
	}
}
