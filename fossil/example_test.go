package fossil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func ExampleScanner_ScanReport() {
	dir, err := os.MkdirTemp("", "mcpkit-fossil-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	fake := filepath.Join(dir, "fake-fossil.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
printf '%s' '[{"rule_id":"DEAD-dead function","title":"unusedOne","description":"never called","severity":"info","confidence":"certain","location":{"file":"main.go","line_start":5,"line_end":5,"column_start":0,"column_end":0},"tags":[],"related_locations":[]}]'
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		panic(err)
	}

	scanner := NewScanner(ScannerConfig{
		Dir:       ".",
		FossilBin: fake,
	})

	report, err := scanner.ScanReport(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("dead_code=%d\n", len(report.FindingsByCategory(CategoryDeadCode)))
	fmt.Printf("first_title=%s\n", report.Findings[0].Title)
	// Output:
	// dead_code=1
	// first_title=unusedOne
}
