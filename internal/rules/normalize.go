package rules

import (
	"fmt"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func normalizeRule(raw Rule) (Rule, error) {
	pattern := strings.ReplaceAll(raw.Pattern, `\`, "/")
	for strings.HasPrefix(pattern, "./") {
		pattern = strings.TrimPrefix(pattern, "./")
	}
	pattern = strings.TrimLeft(pattern, "/")
	directory := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimRight(pattern, "/")

	if pattern == "" || strings.HasPrefix(pattern, "!") {
		return Rule{}, invalidRule(raw)
	}
	if hasParentSegment(pattern) {
		return Rule{}, invalidRule(raw)
	}
	pattern = path.Clean(pattern)
	if pattern == "." {
		return Rule{}, invalidRule(raw)
	}
	if !doublestar.ValidatePattern(pattern) {
		return Rule{}, invalidRule(raw)
	}

	return Rule{
		Pattern:   pattern,
		Source:    raw.Source,
		Line:      raw.Line,
		directory: directory,
		basename:  !strings.Contains(pattern, "/"),
		subtree:   strings.HasSuffix(pattern, "/**"),
	}, nil
}

func normalizePath(candidate string) (string, bool) {
	candidate = strings.ReplaceAll(candidate, `\`, "/")
	for strings.HasPrefix(candidate, "./") {
		candidate = strings.TrimPrefix(candidate, "./")
	}
	if hasParentSegment(candidate) {
		return "", false
	}
	candidate = path.Clean(candidate)
	if candidate == "." || candidate == ".." || strings.HasPrefix(candidate, "../") || strings.HasPrefix(candidate, "/") {
		return "", false
	}
	return candidate, true
}

func hasParentSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func invalidRule(raw Rule) error {
	detail := fmt.Sprintf("invalid exclusion rule %q", raw.Pattern)
	if raw.Source == "" {
		return fmt.Errorf("%s", detail)
	}
	if raw.Line > 0 {
		return fmt.Errorf("invalid rule in %s:%d: %s", raw.Source, raw.Line, detail)
	}
	return fmt.Errorf("invalid rule in %s: %s", raw.Source, detail)
}
