package rules

import (
	"strings"
	"testing"
)

func FuzzRuleNormalizationAndMatching(f *testing.F) {
	seeds := []string{"", ".", "/.", "./.", "/./", "foo/bar", `foo\bar`, "./foo//bar", "/foo/bar", "../escape", "a/../b", "**/.cache/**", "[", "!negation", "日本語/資料", string([]byte{0xff, 'x'})}
	for _, seed := range seeds {
		f.Add(seed, "foo/bar", false)
	}
	f.Fuzz(func(t *testing.T, pattern, candidate string, isDir bool) {
		if len(pattern) > 16*1024 || len(candidate) > 16*1024 {
			t.Skip()
		}
		canonical, err := Canonical(pattern)
		if err != nil {
			return
		}
		if canonical == "" || canonical == "." || strings.Contains("/"+strings.TrimSuffix(canonical, "/")+"/", "/../") || strings.HasPrefix(canonical, "/") || strings.Contains(canonical, `\`) {
			t.Fatalf("unsafe canonical rule %q from %q", canonical, pattern)
		}
		again, err := Canonical(canonical)
		if err != nil || again != canonical {
			t.Fatalf("canonicalization is not idempotent: %q -> %q, %v", canonical, again, err)
		}
		matcher, err := Compile([]string{pattern})
		if err != nil {
			t.Fatalf("canonical rule did not compile: %q: %v", canonical, err)
		}
		first := matcher.Match(candidate, isDir)
		if second := matcher.Match(candidate, isDir); second != first {
			t.Fatalf("nondeterministic match for %q against %q", candidate, canonical)
		}
	})
}

func FuzzIgnoreFileParsing(f *testing.F) {
	for _, seed := range []string{"", "# comment\nnode_modules/\n", "foo\\bar/\r\n*.log\n", "!negation\n", "../escape\n", "日本語/資料\n", "[\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 128*1024 {
			t.Skip()
		}
		parsed, err := parseIgnoreFile(strings.NewReader(input), "fuzz.ignore")
		if err != nil {
			return
		}
		previousLine := 0
		for _, rule := range parsed {
			if rule.Source != "fuzz.ignore" || rule.Line <= previousLine || strings.TrimSpace(rule.Pattern) != rule.Pattern {
				t.Fatalf("incoherent parsed rule: %#v after line %d", rule, previousLine)
			}
			previousLine = rule.Line
		}
		matcher, compileErr := CompileRules(parsed)
		if compileErr != nil {
			return
		}
		for _, rule := range parsed {
			if strings.HasPrefix(rule.Pattern, "!") {
				t.Fatalf("negation unexpectedly accepted: %q", rule.Pattern)
			}
			_ = matcher.Match(rule.Pattern, strings.HasSuffix(rule.Pattern, "/"))
		}
	})
}
