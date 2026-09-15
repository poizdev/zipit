package rules

import (
	"path"

	"github.com/bmatcuk/doublestar/v4"
)

// Matcher is an immutable set of normalized, validated exclusion rules.
type Matcher struct {
	rules []Rule
}

// Compile normalizes and validates raw exclusion patterns once, before a
// filesystem traversal begins.
func Compile(patterns []string) (*Matcher, error) {
	rawRules := make([]Rule, 0, len(patterns))
	for _, pattern := range patterns {
		rawRules = append(rawRules, Rule{Pattern: pattern})
	}
	return CompileRules(rawRules)
}

// CompileRules compiles exclusion rules while preserving source metadata for
// diagnostics.
func CompileRules(rawRules []Rule) (*Matcher, error) {
	matcher := &Matcher{rules: make([]Rule, 0, len(rawRules))}
	for _, raw := range rawRules {
		rule, err := normalizeRule(raw)
		if err != nil {
			return nil, err
		}
		matcher.rules = append(matcher.rules, rule)
	}
	return matcher, nil
}

// Canonical validates a pattern and returns its portable persisted form.
func Canonical(pattern string) (string, error) {
	rule, err := normalizeRule(Rule{Pattern: pattern})
	if err != nil {
		return "", err
	}
	if rule.directory {
		return rule.Pattern + "/", nil
	}
	return rule.Pattern, nil
}

// Match reports whether a normalized root-relative path is excluded. Input is
// defensively normalized so Windows-style separators behave identically on all
// platforms.
func (m *Matcher) Match(candidate string, isDir bool) bool {
	if m == nil {
		return false
	}
	candidate, ok := normalizePath(candidate)
	if !ok {
		return false
	}

	for _, rule := range m.rules {
		if rule.directory && !isDir {
			continue
		}
		if isDir && !rule.directory && !rule.subtree {
			continue
		}
		if rule.basename {
			if doublestar.MatchUnvalidated(rule.Pattern, path.Base(candidate)) {
				return true
			}
			continue
		}

		if doublestar.MatchUnvalidated(rule.Pattern, candidate) {
			return true
		}
		if isDir && doublestar.MatchUnvalidated(rule.Pattern, candidate+"/") {
			return true
		}
	}
	return false
}
