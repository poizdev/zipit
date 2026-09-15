package rules

import (
	"strings"
	"testing"
)

func TestParseIgnoreFilePreservesLineMetadata(t *testing.T) {
	parsed, err := parseIgnoreFile(strings.NewReader("# comment\r\n foo\\bar/ \r\n\r\n*.log\n"), "memory.ignore")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[0].Pattern != `foo\bar/` || parsed[0].Source != "memory.ignore" || parsed[0].Line != 2 || parsed[1].Line != 4 {
		t.Fatalf("parsed rules = %#v", parsed)
	}
}
