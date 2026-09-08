package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/routatic/proxy/internal/cacheusage"
	"github.com/routatic/proxy/internal/history"
	"github.com/routatic/proxy/internal/metrics"
)

func TestHandleMetricsIncludesCacheUsage(t *testing.T) {
	m := metrics.New()
	m.RecordCacheUsage(cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true})
	s := &Server{met: m}

	rr := httptest.NewRecorder()
	s.handleMetrics(rr, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))

	var response metricsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.CacheReadTokens != 75 || response.CacheCreationTokens != 25 {
		t.Fatalf("cache token response = %+v", response)
	}
	if response.CacheRate == nil || *response.CacheRate != 0.75 {
		t.Fatalf("cache rate = %v, want 0.75", response.CacheRate)
	}
}

func TestHandleHistoryIncludesCacheUsage(t *testing.T) {
	hist := history.New(10)
	hist.Add(history.RequestRecord{
		ID:         "request-1",
		Model:      "model-a",
		CacheUsage: cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true},
	})
	s := &Server{hist: hist}

	rr := httptest.NewRecorder()
	s.handleHistory(rr, httptest.NewRequest(http.MethodGet, "/api/history", nil))

	var response []historyEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("history length = %d, want 1", len(response))
	}
	entry := response[0]
	if entry.CacheReadInputTokens == nil || *entry.CacheReadInputTokens != 75 {
		t.Fatalf("cache read = %v, want 75", entry.CacheReadInputTokens)
	}
	if entry.CacheCreationInputTokens == nil || *entry.CacheCreationInputTokens != 25 {
		t.Fatalf("cache creation = %v, want 25", entry.CacheCreationInputTokens)
	}
	if entry.CacheRate == nil || *entry.CacheRate != 0.75 {
		t.Fatalf("cache rate = %v, want 0.75", entry.CacheRate)
	}
}
