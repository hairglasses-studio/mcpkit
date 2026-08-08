// Command fleet-inventory-check is the CI entrypoint for the fleet inventory
// gate: it scans the workspace, diffs a scored result against a committed
// baseline (from fleet_inventory_baseline / fleetinventory.BaselineFromReport),
// and exits non-zero on regression — for wiring into `make ci`.
//
// Exit codes: 0 = pass, 1 = regression (gate failed), 2 = usage/IO error.
//
// A partial workspace does not spuriously fail: repos in the baseline but
// absent on disk are reported as removed, repos present but not in the baseline
// as new — neither is a failure. Only a composite drop beyond the allowed delta,
// a new baseline violation, a new security finding, or a newly Tasks-shape
// non-conformant repo fails the gate.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hairglasses-studio/mcpkit/fleetinventory"
)

func main() {
	root := flag.String("root", defaultRoot(), "workspace root to scan")
	baseline := flag.String("baseline", "", "path to the committed baseline JSON (required)")
	drop := flag.Float64("composite-drop", 0, "allowed composite regression before failing (0 → default 5.0)")
	flag.Parse()

	if *baseline == "" {
		fmt.Fprintln(os.Stderr, "fleet-inventory-check: --baseline is required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*baseline)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fleet-inventory-check: read baseline: %v\n", err)
		os.Exit(2)
	}
	base, err := fleetinventory.ParseBaseline(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fleet-inventory-check: parse baseline: %v\n", err)
		os.Exit(2)
	}

	rep, err := fleetinventory.Scan(context.Background(), *root, fleetinventory.ScanOptions{Score: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fleet-inventory-check: scan: %v\n", err)
		os.Exit(2)
	}

	res := fleetinventory.Check(rep, base, *drop)
	fmt.Print(fleetinventory.RenderCheck(res))
	if !res.Passed {
		os.Exit(1)
	}
}

func defaultRoot() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, "hairglasses-studio")
	}
	return "."
}
