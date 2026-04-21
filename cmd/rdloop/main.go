//go:build !official_sdk

// Command rdloop chains ralph.Loop R&D cycles under a global budget and
// wall-clock duration. It wires rdcycle, research, sampling, roadmap, and
// finops together so an operator can start a multi-cycle autonomous run
// (spec → implement → verify → next-cycle) and stop when either the dollar
// budget or the duration elapses. State is persisted to .rdloop_state.json
// so a crash mid-run can resume.
//
// The main() entry is currently a stub that exits non-zero — see
// runner.go for the runtime that ships once this command is re-enabled.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "rdloop is disabled to conserve Anthropic budget for active coding sessions")
	os.Exit(1)
}
