package gui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/routatic/proxy/internal/storage"
)

// AnalyticsHandler serves analytics endpoints for the dashboard.
type AnalyticsHandler struct {
	store   *storage.Analytics
	latency *storage.Latency
}

// NewAnalyticsHandler creates a handler backed by the given database.
// It internally creates an Analytics store and a Latency store.
func NewAnalyticsHandler(db *storage.Database) *AnalyticsHandler {
	return &AnalyticsHandler{
		store:   storage.NewAnalytics(db),
		latency: storage.NewLatency(db),
	}
}

func (h *AnalyticsHandler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *AnalyticsHandler) getRange(r *http.Request) (time.Time, int, string) {
	if r.URL.Query().Get("range") == "today" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), 0, "today"
	}

	daysStr := r.URL.Query().Get("days")
	if daysStr == "" {
		return time.Now().AddDate(0, 0, -30), 30, "days"
	}
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}
	return time.Now().AddDate(0, 0, -days), days, "days"
}

// Summary returns high-level KPIs and breakdowns.
func (h *AnalyticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	since, _, period := h.getRange(r)

	summary, err := h.store.GetTokenSummarySince(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	models, err := h.store.GetModelBreakdownSince(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	providers, err := h.store.GetProviderBreakdownSince(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"summary":      summary,
		"models":       models,
		"providers":    providers,
		"range":        period,
		"generated_at": time.Now().Format(time.RFC3339),
	}
	h.writeJSON(w, resp)
}

// TokenTrend returns daily aggregates for multi-day ranges and hourly
// aggregates for Today.
func (h *AnalyticsHandler) TokenTrend(w http.ResponseWriter, r *http.Request) {
	since, days, period := h.getRange(r)
	var (
		trend []storage.DailyTokenPoint
		err   error
	)
	if period == "today" {
		trend, err = h.store.GetHourlyTokenTrendSince(since)
	} else {
		trend, err = h.store.GetDailyTokenTrendSince(since)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, map[string]any{
		"days":  days,
		"range": period,
		"trend": trend,
	})
}

// LatencyStats returns latency stats per model.
func (h *AnalyticsHandler) LatencyStats(w http.ResponseWriter, r *http.Request) {
	since, days, period := h.getRange(r)
	stats, err := h.latency.GetStats(since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, map[string]any{
		"days":  days,
		"range": period,
		"stats": stats,
	})
}
