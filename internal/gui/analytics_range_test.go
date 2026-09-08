package gui

import (
	"net/http/httptest"
	"testing"
	"time"
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
