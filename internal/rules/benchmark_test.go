package rules

import "testing"

func BenchmarkMatcherRealisticRules(b *testing.B) {
	matcher, err := Compile([]string{".git/", "node_modules/", ".DS_Store", "Thumbs.db", "*.log", "**/.cache/**", "build/", "recordings/", "generated/**"})
	if err != nil {
		b.Fatal(err)
	}
	paths := []string{"auth-layer/src/main.go", "auth-layer/node_modules/pkg/index.js", "devtv/recordings/episode.mp4", "devcast/.cache/state", "README.md", "services/api/debug.log", "generated/client/api.go", "日本語/資料.txt"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, candidate := range paths {
			_ = matcher.Match(candidate, false)
		}
	}
}
