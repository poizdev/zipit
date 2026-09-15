package update

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		want    string
		wantErr bool
	}{
		{name: "with v prefix", tag: "v0.1.0", want: "v0.1.0"},
		{name: "without v prefix", tag: "0.1.0", want: "v0.1.0"},
		{name: "major only", tag: "v1.0.0", want: "v1.0.0"},
		{name: "double digit", tag: "v0.10.0", want: "v0.10.0"},
		{name: "prerelease", tag: "v0.2.0-beta.1", want: "v0.2.0-beta.1"},
		{name: "prerelease rc", tag: "v0.2.0-rc.1", want: "v0.2.0-rc.1"},
		{name: "build metadata", tag: "v1.0.0+build123", want: "v1.0.0"},
		{name: "empty", tag: "", wantErr: true},
		{name: "garbage", tag: "notaversion", wantErr: true},
		{name: "dev", tag: "dev", wantErr: true},
		{name: "partial major", tag: "v1", want: "v1.0.0"},
		{name: "partial major minor", tag: "v1.0", want: "v1.0.0"},
		{name: "leading zeros", tag: "v01.0.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) = %q, want error", tt.tag, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q) error = %v", tt.tag, err)
			}
			if got != tt.want {
				t.Fatalf("ParseVersion(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{name: "equal", a: "v0.1.0", b: "v0.1.0", want: 0},
		{name: "patch newer", a: "v0.1.1", b: "v0.1.0", want: 1},
		{name: "patch older", a: "v0.1.0", b: "v0.1.1", want: -1},
		{name: "minor newer", a: "v0.2.0", b: "v0.1.99", want: 1},
		{name: "double digit minor", a: "v0.10.0", b: "v0.9.9", want: 1},
		{name: "major newer", a: "v1.0.0", b: "v0.99.99", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsNewerStable(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
	}{
		{name: "same version", current: "v0.1.0", candidate: "v0.1.0", want: false},
		{name: "newer patch", current: "v0.1.0", candidate: "v0.1.1", want: true},
		{name: "newer minor", current: "v0.1.0", candidate: "v0.2.0", want: true},
		{name: "newer major", current: "v0.1.0", candidate: "v1.0.0", want: true},
		{name: "older version", current: "v0.2.0", candidate: "v0.1.0", want: false},
		{name: "double digit minor", current: "v0.9.9", candidate: "v0.10.0", want: true},
		{name: "prerelease candidate", current: "v0.1.0", candidate: "v0.2.0-beta.1", want: false},
		{name: "prerelease rc", current: "v0.1.0", candidate: "v0.2.0-rc.1", want: false},
		{name: "without v prefix current", current: "0.1.0", candidate: "v0.2.0", want: true},
		{name: "without v prefix candidate", current: "v0.1.0", candidate: "0.2.0", want: true},
		{name: "both without v prefix", current: "0.1.0", candidate: "0.2.0", want: true},
		{name: "malformed current", current: "garbage", candidate: "v0.2.0", want: false},
		{name: "malformed candidate", current: "v0.1.0", candidate: "garbage", want: false},
		{name: "dev current", current: "dev", candidate: "v0.1.0", want: false},
		{name: "empty current", current: "", candidate: "v0.1.0", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewerStable(tt.current, tt.candidate)
			if got != tt.want {
				t.Fatalf("IsNewerStable(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
			}
		})
	}
}

func TestIsDevVersion(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"dev", true},
		{"", true},
		{"v0.1.0", false},
		{"0.1.0", false},
		{"v1.0.0-rc.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			if got := IsDevVersion(tt.v); got != tt.want {
				t.Fatalf("IsDevVersion(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestIsPrerelease(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"v0.1.0", false},
		{"v1.0.0", false},
		{"v0.2.0-beta.1", true},
		{"v0.2.0-rc.1", true},
		{"0.2.0-alpha.1", true},
		{"0.1.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			if got := IsPrerelease(tt.v); got != tt.want {
				t.Fatalf("IsPrerelease(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestInvalidVersionError(t *testing.T) {
	err := &InvalidVersionError{Tag: "notaversion"}
	if err.Error() != "invalid version: notaversion" {
		t.Fatalf("Error() = %q", err.Error())
	}
}
