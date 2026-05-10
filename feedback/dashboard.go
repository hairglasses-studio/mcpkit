package feedback

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// DashboardExport is a stable JSON shape for dashboard data sources.
type DashboardExport struct {
	SchemaVersion int            `json:"schema_version"`
	Source        string         `json:"source,omitempty"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Rows          []DashboardRow `json:"rows"`
}

// DashboardRow is one package/tool aggregate row.
type DashboardRow struct {
	Target          string    `json:"target"`
	Package         string    `json:"package"`
	Tool            string    `json:"tool"`
	Calls           int64     `json:"calls"`
	Errors          int64     `json:"errors"`
	ErrorRate       float64   `json:"error_rate"`
	LastErrorCode   string    `json:"last_error_code,omitempty"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	LastSeenUnixMS  int64     `json:"last_seen_unix_ms"`
	GeneratedUnixMS int64     `json:"generated_unix_ms"`
}

// DashboardExport converts a telemetry snapshot into dashboard-friendly rows.
func (s TelemetrySnapshot) DashboardExport() DashboardExport {
	rows := make([]DashboardRow, 0, len(s.Counters))
	generatedUnixMS := s.GeneratedAt.UnixMilli()
	for _, counter := range s.Counters {
		rows = append(rows, DashboardRow{
			Target:          dashboardTarget(counter.Package, counter.Tool),
			Package:         counter.Package,
			Tool:            counter.Tool,
			Calls:           counter.Calls,
			Errors:          counter.Errors,
			ErrorRate:       counter.ErrorRate,
			LastErrorCode:   counter.LastErrorCode,
			LastSeenAt:      counter.LastSeenAt,
			LastSeenUnixMS:  counter.LastSeenAt.UnixMilli(),
			GeneratedUnixMS: generatedUnixMS,
		})
	}
	return DashboardExport{
		SchemaVersion: 1,
		Source:        s.Source,
		GeneratedAt:   s.GeneratedAt,
		Rows:          rows,
	}
}

// DashboardExport returns a dashboard-friendly JSON export from the collector's
// current snapshot.
func (c *TelemetryCollector) DashboardExport() DashboardExport {
	return c.Snapshot().DashboardExport()
}

// DashboardJSON marshals the current telemetry dashboard export.
func (c *TelemetryCollector) DashboardJSON() ([]byte, error) {
	return json.MarshalIndent(c.DashboardExport(), "", "  ")
}

// DashboardHandler serves the collector dashboard export as JSON.
func DashboardHandler(collector *TelemetryCollector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		c := collector
		if c == nil {
			c, _ = NewTelemetryCollector(TelemetryConfig{})
		}
		if err := json.NewEncoder(w).Encode(c.DashboardExport()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func dashboardTarget(pkg, tool string) string {
	pkg = strings.TrimSpace(pkg)
	tool = strings.TrimSpace(tool)
	switch {
	case pkg == "" && tool == "":
		return "unknown"
	case pkg == "":
		return tool
	case tool == "":
		return pkg
	default:
		return pkg + "." + tool
	}
}
