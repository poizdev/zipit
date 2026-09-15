//go:build !windows

package cli

import (
	"syscall"
	"testing"
)

func TestInterruptSignalsIncludeSIGTERMOnUnix(t *testing.T) {
	for _, signal := range interruptSignals() {
		if signal == syscall.SIGTERM {
			return
		}
	}
	t.Fatal("interrupt signals do not include SIGTERM")
}
