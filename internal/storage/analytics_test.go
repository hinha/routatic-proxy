package storage

import (
	"math"
	"testing"
	"time"

	"github.com/routatic/proxy/internal/cacheusage"
	"github.com/routatic/proxy/internal/history"
)

func TestAnalyticsAggregatesReportedCacheUsage(t *testing.T) {
	db := openTestDatabase(t)
	requests := NewRequests(db)
	now := time.Now().UTC()

	for _, rec := range []history.RequestRecord{
		{
			ID:          "hit-and-create",
			Model:       "model-a",
			Provider:    "provider-a",
			StartTime:   now,
			InputTokens: 20,
			Success:     true,
			CacheUsage:  cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true},
		},
		{
			ID:          "hit-only",
			Model:       "model-a",
			Provider:    "provider-a",
			StartTime:   now,
			InputTokens: 10,
			Success:     true,
			CacheUsage:  cacheusage.Usage{ReadTokens: 25, Reported: true},
		},
		{
			ID:          "unreported",
			Model:       "model-a",
			Provider:    "provider-a",
			StartTime:   now,
			InputTokens: 30,
			Success:     true,
		},
	} {
		if err := requests.Insert(rec); err != nil {
			t.Fatalf("Insert(%s) error = %v", rec.ID, err)
		}
	}

	analytics := NewAnalytics(db)
	summary, err := analytics.GetTokenSummary(30)
	if err != nil {
		t.Fatalf("GetTokenSummary() error = %v", err)
	}
	if got, want := summary.CacheReadTokens, int64(100); got != want {
		t.Fatalf("CacheReadTokens = %d, want %d", got, want)
	}
	if got, want := summary.CacheCreationTokens, int64(25); got != want {
		t.Fatalf("CacheCreationTokens = %d, want %d", got, want)
	}
	if got, want := summary.CacheUsageRequests, int64(2); got != want {
		t.Fatalf("CacheUsageRequests = %d, want %d", got, want)
	}
	if got, want := *summary.CacheRate, 100.0/125.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("CacheRate = %v, want %v", got, want)
	}

	models, err := analytics.GetModelBreakdown(30)
	if err != nil {
		t.Fatalf("GetModelBreakdown() error = %v", err)
	}
	if len(models) != 1 || models[0].CacheReadTokens != 100 || models[0].CacheCreationTokens != 25 {
		t.Fatalf("model cache metrics = %+v, want read=100 creation=25", models)
	}

	providers, err := analytics.GetProviderBreakdown(30)
	if err != nil {
		t.Fatalf("GetProviderBreakdown() error = %v", err)
	}
	if len(providers) != 1 || providers[0].CacheReadTokens != 100 || providers[0].CacheCreationTokens != 25 {
		t.Fatalf("provider cache metrics = %+v, want read=100 creation=25", providers)
	}

	trend, err := analytics.GetDailyTokenTrend(30)
	if err != nil {
		t.Fatalf("GetDailyTokenTrend() error = %v", err)
	}
	if len(trend) != 1 || trend[0].CacheReadTokens != 100 || trend[0].CacheCreationTokens != 25 {
		t.Fatalf("daily cache metrics = %+v, want read=100 creation=25", trend)
	}
}

func TestAnalyticsSinceIncludesRequestsFromToday(t *testing.T) {
	db := openTestDatabase(t)
	requests := NewRequests(db)
	now := time.Now()
	if err := requests.Insert(history.RequestRecord{
		ID:        "today-request",
		Model:     "model-today",
		StartTime: now,
		Success:   true,
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	summary, err := NewAnalytics(db).GetTokenSummarySince(startOfDay)
	if err != nil {
		t.Fatalf("GetTokenSummarySince() error = %v", err)
	}
	if summary.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", summary.TotalRequests)
	}
}

func TestAnalyticsHourlyTrendGroupsRequestsByHour(t *testing.T) {
	db := openTestDatabase(t)
	requests := NewRequests(db)
	location := time.FixedZone("WIB", 7*60*60)
	day := time.Date(2026, time.September, 8, 0, 0, 0, 0, location)

	for _, rec := range []history.RequestRecord{
		{ID: "hour-09-a", Model: "model", StartTime: day.Add(9*time.Hour + 10*time.Minute), InputTokens: 10, Success: true},
		{ID: "hour-09-b", Model: "model", StartTime: day.Add(9*time.Hour + 45*time.Minute), InputTokens: 20, Success: true},
		{ID: "hour-10", Model: "model", StartTime: day.Add(10*time.Hour + 5*time.Minute), InputTokens: 30, Success: true},
	} {
		if err := requests.Insert(rec); err != nil {
			t.Fatalf("Insert(%s) error = %v", rec.ID, err)
		}
	}

	trend, err := NewAnalytics(db).GetHourlyTokenTrendSince(day)
	if err != nil {
		t.Fatalf("GetHourlyTokenTrendSince() error = %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("hourly trend length = %d, want 2: %+v", len(trend), trend)
	}
	if got, want := trend[0].Date, "2026-09-08T09:00:00"; got != want {
		t.Fatalf("first bucket = %q, want %q", got, want)
	}
	if got, want := trend[0].Requests, int64(2); got != want {
		t.Fatalf("first bucket requests = %d, want %d", got, want)
	}
	if got, want := trend[1].Date, "2026-09-08T10:00:00"; got != want {
		t.Fatalf("second bucket = %q, want %q", got, want)
	}
}
