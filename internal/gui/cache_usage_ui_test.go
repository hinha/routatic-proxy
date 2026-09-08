package gui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestDashboardContainsCacheUsageSurfaces(t *testing.T) {
	index, err := fs.ReadFile(assets, "assets/index.html")
	if err != nil {
		t.Fatalf("read dashboard HTML: %v", err)
	}
	app, err := fs.ReadFile(assets, "assets/app.js")
	if err != nil {
		t.Fatalf("read dashboard JavaScript: %v", err)
	}

	for _, needle := range []string{
		`value="today"`,
		"analytics.today",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
		"cache_rate",
		"cache-breakdown-tbody",
		"kpi-cache-read",
		"kpi-cache-created",
		"kpi-cache-rate",
	} {
		if !strings.Contains(string(index), needle) && !strings.Contains(string(app), needle) {
			t.Errorf("dashboard assets missing %q", needle)
		}
	}
}
