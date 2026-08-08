package fleetinventory

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// VulnDBModulesURL is the Go vuln DB index of module→advisories.
const VulnDBModulesURL = "https://vuln.go.dev/index/modules.json"

// FetchVulnDBModules downloads the Go vuln DB modules index to destPath. This
// is the OUT-OF-BAND refresh step (a weekly cron / make target) — never called
// during a scan; scans read the cached snapshot via ScanOptions.VulnDBPath.
func FetchVulnDBModules(destPath string) error {
	if destPath == "" {
		return fmt.Errorf("empty destination path")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(VulnDBModulesURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", VulnDBModulesURL, resp.StatusCode)
	}
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, destPath) // atomic swap
}
