package main

import (
	"errors"
	"testing"

	"github.com/poizdev/zipit/internal/cli"
)

func TestExitCodeUsesConventionalInterruptStatus(t *testing.T) {
	if got := exitCode(cli.ErrInterrupted); got != 130 {
		t.Fatalf("exitCode(interrupted) = %d, want 130", got)
	}
	if got := exitCode(errors.New("failed")); got != 1 {
		t.Fatalf("exitCode(failed) = %d, want 1", got)
	}
}
