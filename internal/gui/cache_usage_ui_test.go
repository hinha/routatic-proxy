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
	style, err := fs.ReadFile(assets, "assets/style.css")
	if err != nil {
		t.Fatalf("read dashboard CSS: %v", err)
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
		"analytics-kpi-grid",
	} {
		if !strings.Contains(string(index), needle) && !strings.Contains(string(app), needle) && !strings.Contains(string(style), needle) {
			t.Errorf("dashboard assets missing %q", needle)
		}
	}

	for _, needle := range []string{
		"trend.range",
		"trend-axis-labels",
		"formatTrendLabel",
		"trend-chart-stack",
		"trend-hit-target",
		"trend-tooltip",
		"trend-selection",
		"bindTrendInteractions",
		"keydown",
		"donut-segment",
		"donut-tooltip",
		"donut-selection",
		"bindDonutInteractions",
		".donut-tooltip[hidden]",
		".donut-selection[hidden]",
	} {
		if !strings.Contains(string(index), needle) && !strings.Contains(string(app), needle) && !strings.Contains(string(style), needle) {
			t.Errorf("dashboard assets missing %q", needle)
		}
	}

	for _, removed := range []string{"kpi-cost", "kpi-p95", "Est. Cost", "p95 Latency", "/api/analytics/latency"} {
		if strings.Contains(string(index), removed) || strings.Contains(string(app), removed) {
			t.Errorf("dashboard still contains removed empty KPI %q", removed)
		}
	}
}
