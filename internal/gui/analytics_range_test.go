package gui

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/routatic/proxy/internal/history"
	"github.com/routatic/proxy/internal/storage"
)

func TestAnalyticsRangeTodayStartsAtLocalMidnight(t *testing.T) {
	now := time.Now()
	h := &AnalyticsHandler{}
	since, days, period := h.getRange(httptest.NewRequest("GET", "/api/analytics/summary?range=today", nil))

	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !since.Equal(want) {
		t.Fatalf("today start = %v, want %v", since, want)
	}
	if days != 0 || period != "today" {
		t.Fatalf("today range = (%d, %q), want (0, %q)", days, period, "today")
	}
}

func TestAnalyticsRangeDefaultsToThirtyDays(t *testing.T) {
	now := time.Now()
	h := &AnalyticsHandler{}
	since, days, period := h.getRange(httptest.NewRequest("GET", "/api/analytics/summary", nil))

	want := now.AddDate(0, 0, -30)
	if since.Before(want.Add(-time.Second)) || since.After(want.Add(time.Second)) {
		t.Fatalf("default start = %v, want approximately %v", since, want)
	}
	if days != 30 || period != "days" {
		t.Fatalf("default range = (%d, %q), want (30, %q)", days, period, "days")
	}
}

func TestAnalyticsTokenTrendTodayUsesHourlyBuckets(t *testing.T) {
	db, err := storage.Open(storage.Config{DatabasePath: filepath.Join(t.TempDir(), "analytics.db")})
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := storage.NewRequests(db).Insert(history.RequestRecord{
		ID:        "today-hour",
		Model:     "model",
		StartTime: time.Now().Add(-5 * time.Minute),
		Success:   true,
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	rr := httptest.NewRecorder()
	NewAnalyticsHandler(db).TokenTrend(rr, httptest.NewRequest("GET", "/api/analytics/tokens/trend?range=today", nil))
	if rr.Code != 200 {
		t.Fatalf("TokenTrend() status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}

	var response struct {
		Range string `json:"range"`
		Trend []struct {
			Date string `json:"date"`
		} `json:"trend"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode trend response: %v", err)
	}
	if response.Range != "today" || len(response.Trend) != 1 {
		t.Fatalf("trend response = %+v, want today with one bucket", response)
	}
	if !strings.Contains(response.Trend[0].Date, "T") {
		t.Fatalf("today trend date = %q, want hourly timestamp", response.Trend[0].Date)
	}
}
