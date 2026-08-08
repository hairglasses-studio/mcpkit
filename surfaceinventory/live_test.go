package surfaceinventory

import (
	"reflect"
	"testing"
)

func TestDiffNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		live   []string
		static []string
		want   LiveDiff
	}{
		{
			name:   "identical sets",
			live:   []string{"a", "b", "c"},
			static: []string{"c", "a", "b"},
			want: LiveDiff{
				LiveOnly:   nil,
				StaticOnly: nil,
				Both:       []string{"a", "b", "c"},
				Counts:     LiveDiffCounts{Live: 3, Static: 3, LiveOnly: 0, StaticOnly: 0, Both: 3},
			},
		},
		{
			name:   "live has an extra tool the scanner missed",
			live:   []string{"a", "b", "dynamic_registered"},
			static: []string{"a", "b"},
			want: LiveDiff{
				LiveOnly:   []string{"dynamic_registered"},
				StaticOnly: nil,
				Both:       []string{"a", "b"},
				Counts:     LiveDiffCounts{Live: 3, Static: 2, LiveOnly: 1, StaticOnly: 0, Both: 2},
			},
		},
		{
			name:   "static scan found a tool never actually registered at runtime",
			live:   []string{"a"},
			static: []string{"a", "conditionally_registered"},
			want: LiveDiff{
				LiveOnly:   nil,
				StaticOnly: []string{"conditionally_registered"},
				Both:       []string{"a"},
				Counts:     LiveDiffCounts{Live: 1, Static: 2, LiveOnly: 0, StaticOnly: 1, Both: 1},
			},
		},
		{
			name:   "completely disjoint",
			live:   []string{"x", "y"},
			static: []string{"p", "q"},
			want: LiveDiff{
				LiveOnly:   []string{"x", "y"},
				StaticOnly: []string{"p", "q"},
				Both:       nil,
				Counts:     LiveDiffCounts{Live: 2, Static: 2, LiveOnly: 2, StaticOnly: 2, Both: 0},
			},
		},
		{
			name:   "both empty",
			live:   nil,
			static: nil,
			want:   LiveDiff{Counts: LiveDiffCounts{}},
		},
		{
			name:   "duplicates in either input are deduplicated",
			live:   []string{"a", "a", "b"},
			static: []string{"a", "a", "a"},
			want: LiveDiff{
				LiveOnly:   []string{"b"},
				StaticOnly: nil,
				Both:       []string{"a"},
				Counts:     LiveDiffCounts{Live: 2, Static: 1, LiveOnly: 1, StaticOnly: 0, Both: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := diffNames(tt.live, tt.static)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("diffNames(%v, %v) = %+v, want %+v", tt.live, tt.static, got, tt.want)
			}
		})
	}
}

func TestStaticNamesForKind_SingleRepoDir(t *testing.T) {
	t.Parallel()

	// surfaceinventory itself is not a workspace root (no manifest.json, no
	// nested .git subdirs) — staticNamesForKind must fall back to treating
	// it as a single repo and still find its own tool registrations.
	names, err := staticNamesForKind(".", nil, KindMCPTool)
	if err != nil {
		t.Fatalf("staticNamesForKind: %v", err)
	}

	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"surface_inventory_scan", "surface_inventory_report", "surface_inventory_live"} {
		if !got[want] {
			t.Errorf("staticNamesForKind(\".\") = %v, missing %q", names, want)
		}
	}
}

func TestLiveToolDef_Registration(t *testing.T) {
	t.Parallel()

	td := liveToolDef()
	if td.Tool.Name != "surface_inventory_live" {
		t.Errorf("Name = %q, want surface_inventory_live", td.Tool.Name)
	}
	if td.Tool.Description == "" {
		t.Error("Description should not be empty")
	}
	if td.Category != "audit" {
		t.Errorf("Category = %q, want audit", td.Category)
	}
	if td.Handler == nil {
		t.Error("Handler should not be nil")
	}
}

func TestModule_IncludesLiveTool(t *testing.T) {
	t.Parallel()

	tools := Module().Tools()
	found := false
	for _, td := range tools {
		if td.Tool.Name == "surface_inventory_live" {
			found = true
		}
	}
	if !found {
		t.Errorf("Module().Tools() = %d tools, missing surface_inventory_live", len(tools))
	}
}
