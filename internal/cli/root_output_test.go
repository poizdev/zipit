package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/poizdev/zipit/internal/archive"
)

func TestSummariesReportUnsupportedFilesystemEntriesCompactly(t *testing.T) {
	result := archive.Result{Stats: archive.Stats{SpecialFilesSkipped: 2, SymlinksIncluded: 3}}
	for _, render := range []func(*bytes.Buffer) error{
		func(output *bytes.Buffer) error {
			return printDryRunSummary(output, "/tmp/archive.zip", false, result, time.Second)
		},
		func(output *bytes.Buffer) error {
			return printArchiveSummary(output, "/tmp/archive.zip", result, time.Second)
		},
	} {
		var output bytes.Buffer
		if err := render(&output); err != nil {
			t.Fatal(err)
		}
		if strings.Count(output.String(), "Skipped 2 unsupported filesystem entries") != 1 {
			t.Fatalf("output = %q", output.String())
		}
		if strings.Count(output.String(), "Preserved 3 symlinks") != 1 {
			t.Fatalf("output = %q", output.String())
		}
	}
}
