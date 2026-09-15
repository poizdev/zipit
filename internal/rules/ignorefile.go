package rules

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const ignoreFileName = ".zipitignore"

// LoadIgnoreFile reads exclusion rules from the source root's .zipitignore.
// A missing file is a valid empty rule set. Nested files are not discovered.
func LoadIgnoreFile(sourceRoot string) ([]Rule, error) {
	ignorePath := filepath.Join(sourceRoot, ignoreFileName)
	file, err := os.Open(ignorePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ignorePath, err)
	}
	defer file.Close()

	return parseIgnoreFile(file, ignorePath)
}

func parseIgnoreFile(reader io.Reader, source string) ([]Rule, error) {
	var loaded []Rule
	scanner := bufio.NewScanner(reader)
	for line := 1; scanner.Scan(); line++ {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		loaded = append(loaded, Rule{Pattern: pattern, Source: source, Line: line})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}
	return loaded, nil
}
