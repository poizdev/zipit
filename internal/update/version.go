package update

import "golang.org/x/mod/semver"

// ParseVersion normalizes a version tag to canonical semver form.
// It ensures a "v" prefix and validates the result.
func ParseVersion(tag string) (string, error) {
	v := tag
	if len(v) > 0 && v[0] != 'v' {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return "", &InvalidVersionError{Tag: tag}
	}
	return semver.Canonical(v), nil
}

// CompareVersions returns -1, 0, or +1 comparing a to b.
// Both must be valid semver strings with "v" prefix.
func CompareVersions(a, b string) int {
	return semver.Compare(a, b)
}

// IsNewerStable reports whether candidate is a newer stable version than current.
// Both are raw version strings (with or without "v" prefix).
// Returns false if either is invalid or if candidate is a prerelease.
func IsNewerStable(current, candidate string) bool {
	cur, err := ParseVersion(current)
	if err != nil {
		return false
	}
	cand, err := ParseVersion(candidate)
	if err != nil {
		return false
	}
	if IsPrerelease(cand) {
		return false
	}
	return semver.Compare(cand, cur) > 0
}

// IsDevVersion reports whether v represents a development build.
func IsDevVersion(v string) bool {
	return v == "" || v == "dev"
}

// IsPrerelease reports whether v has a prerelease suffix.
func IsPrerelease(v string) bool {
	if len(v) > 0 && v[0] != 'v' {
		v = "v" + v
	}
	return semver.Prerelease(v) != ""
}

// InvalidVersionError describes a version string that is not valid semver.
type InvalidVersionError struct {
	Tag string
}

func (e *InvalidVersionError) Error() string {
	return "invalid version: " + e.Tag
}
