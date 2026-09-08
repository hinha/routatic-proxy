package storage

import (
	"context"
	"time"
)

// Analytics provides aggregated metrics for the dashboard.
type Analytics struct {
	db *Database
}

// NewAnalytics creates a new Analytics store.
func NewAnalytics(db *Database) *Analytics {
	return &Analytics{db: db}
}

// CacheMetrics holds provider-reported prompt-cache usage for a time window.
type CacheMetrics struct {
	CacheReadTokens     int64    `json:"cache_read_tokens"`
	CacheCreationTokens int64    `json:"cache_creation_tokens"`
	CacheUsageRequests  int64    `json:"cache_usage_requests"`
	CacheRate           *float64 `json:"cache_rate"`
}

func newCacheMetrics(readTokens, creationTokens, reportedRequests int64) CacheMetrics {
	metrics := CacheMetrics{
		CacheReadTokens:     readTokens,
		CacheCreationTokens: creationTokens,
		CacheUsageRequests:  reportedRequests,
	}
	total := readTokens + creationTokens
	if total > 0 {
		rate := float64(readTokens) / float64(total)
		metrics.CacheRate = &rate
	}
	return metrics
}

func sinceForDays(days int) time.Time {
	if days <= 0 {
		days = 30
	}
	return time.Now().AddDate(0, 0, -days)
}

// TokenSummary holds high-level token and request metrics for a time window.
type TokenSummary struct {
	TotalRequests int64     `json:"total_requests"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	SuccessRate   float64   `json:"success_rate"` // 0-1
	EstCostUSD    float64   `json:"est_cost_usd"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	CacheMetrics
}

// GetTokenSummary returns aggregated token/request metrics for the last N days.
func (a *Analytics) GetTokenSummary(days int) (*TokenSummary, error) {
	return a.GetTokenSummarySince(sinceForDays(days))
}

// GetTokenSummarySince returns aggregated metrics from the given start time.
func (a *Analytics) GetTokenSummarySince(since time.Time) (*TokenSummary, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var summary TokenSummary
	summary.PeriodStart = since
	summary.PeriodEnd = time.Now()

	row := a.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total_requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_read_input_tokens, 0) ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_creation_input_tokens, 0) ELSE 0 END), 0) AS cache_creation_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN 1 ELSE 0 END), 0) AS cache_usage_requests,
			CASE
				WHEN COUNT(*) > 0 THEN CAST(SUM(success) AS FLOAT) / COUNT(*)
				ELSE 0
			END AS success_rate,
			COALESCE(
				(SUM(input_tokens * COALESCE(m.cost_input_per_m, 0)) +
				 SUM(output_tokens * COALESCE(m.cost_output_per_m, 0))) / 1000000,
				0
			) AS est_cost_usd
		FROM requests r
		LEFT JOIN models m ON m.id = r.model
		WHERE datetime(r.start_time) >= datetime(?)
	`, since.Format(time.RFC3339Nano))

	var cacheRead, cacheCreation, cacheRequests int64
	if err := row.Scan(&summary.TotalRequests, &summary.InputTokens, &summary.OutputTokens, &cacheRead, &cacheCreation, &cacheRequests, &summary.SuccessRate, &summary.EstCostUSD); err != nil {
		return nil, err
	}
	summary.CacheMetrics = newCacheMetrics(cacheRead, cacheCreation, cacheRequests)
	return &summary, nil
}

// ModelBreakdown holds per-model usage and performance stats.
type ModelBreakdown struct {
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	EstCostUSD   float64 `json:"est_cost_usd"` // based on models.cost_* if available
	CacheMetrics
}

// GetModelBreakdown returns usage stats per model for the last N days.
func (a *Analytics) GetModelBreakdown(days int) ([]ModelBreakdown, error) {
	return a.GetModelBreakdownSince(sinceForDays(days))
}

// GetModelBreakdownSince returns per-model aggregates from the given start time.
func (a *Analytics) GetModelBreakdownSince(since time.Time) ([]ModelBreakdown, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := a.db.DB().QueryContext(ctx, `
		SELECT 
			r.model,
			COALESCE(r.provider, '') AS provider,
			COUNT(*) AS requests,
			COALESCE(SUM(r.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(r.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN r.cache_usage_reported = 1 THEN COALESCE(r.cache_read_input_tokens, 0) ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN r.cache_usage_reported = 1 THEN COALESCE(r.cache_creation_input_tokens, 0) ELSE 0 END), 0) AS cache_creation_tokens,
			COALESCE(SUM(CASE WHEN r.cache_usage_reported = 1 THEN 1 ELSE 0 END), 0) AS cache_usage_requests,
			COALESCE(AVG(l.latency_ms), 0) AS avg_latency_ms,
			CASE 
				WHEN COUNT(*) > 0 THEN CAST(SUM(r.success) AS FLOAT) / COUNT(*)
				ELSE 0 
			END AS success_rate,
			COALESCE(
				(SUM(r.input_tokens * COALESCE(m.cost_input_per_m, 0)) + 
				 SUM(r.output_tokens * COALESCE(m.cost_output_per_m, 0))) / 1000000,
				0
			) AS est_cost_usd
		FROM requests r
		LEFT JOIN latency_samples l 
			ON l.model = r.model 
			AND datetime(l.recorded_at) >= datetime(?)
		LEFT JOIN models m 
			ON m.id = r.model
		WHERE datetime(r.start_time) >= datetime(?)
		GROUP BY r.model, r.provider
		ORDER BY requests DESC
	`, since.Format(time.RFC3339Nano), since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []ModelBreakdown
	for rows.Next() {
		var mb ModelBreakdown
		var cacheRead, cacheCreation, cacheRequests int64
		if err := rows.Scan(
			&mb.Model,
			&mb.Provider,
			&mb.Requests,
			&mb.InputTokens,
			&mb.OutputTokens,
			&cacheRead,
			&cacheCreation,
			&cacheRequests,
			&mb.AvgLatencyMs,
			&mb.SuccessRate,
			&mb.EstCostUSD,
		); err != nil {
			return nil, err
		}
		mb.CacheMetrics = newCacheMetrics(cacheRead, cacheCreation, cacheRequests)
		result = append(result, mb)
	}
	return result, rows.Err()
}

// ProviderBreakdown holds per-provider aggregates.
type ProviderBreakdown struct {
	Provider     string  `json:"provider"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	FallbackRate float64 `json:"fallback_rate"` // % of requests that were fallbacks
	EstCostUSD   float64 `json:"est_cost_usd"`
	CacheMetrics
}

// GetProviderBreakdown returns usage by provider (with fallback rate).
func (a *Analytics) GetProviderBreakdown(days int) ([]ProviderBreakdown, error) {
	return a.GetProviderBreakdownSince(sinceForDays(days))
}

// GetProviderBreakdownSince returns per-provider aggregates from the given start time.
func (a *Analytics) GetProviderBreakdownSince(since time.Time) ([]ProviderBreakdown, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := a.db.DB().QueryContext(ctx, `
		SELECT 
			COALESCE(provider, 'unknown') AS provider,
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_read_input_tokens, 0) ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_creation_input_tokens, 0) ELSE 0 END), 0) AS cache_creation_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN 1 ELSE 0 END), 0) AS cache_usage_requests,
			COALESCE(
				(SUM(input_tokens) * 0 + SUM(output_tokens) * 0) / 1000000, -- placeholder; real cost via model join if needed
				0
			) AS est_cost_usd,
			-- Fallback rate: attempts > 1 are fallbacks; old rows (NULL/0) treated as primary (rate 0)
			COALESCE(100.0 * COUNT(CASE WHEN COALESCE(attempt, 1) > 1 THEN 1 END) / NULLIF(COUNT(*), 0), 0) AS fallback_rate
		FROM requests
		WHERE datetime(start_time) >= datetime(?)
		GROUP BY provider
		ORDER BY requests DESC
	`, since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []ProviderBreakdown
	for rows.Next() {
		var pb ProviderBreakdown
		var cacheRead, cacheCreation, cacheRequests int64
		if err := rows.Scan(
			&pb.Provider,
			&pb.Requests,
			&pb.InputTokens,
			&pb.OutputTokens,
			&cacheRead,
			&cacheCreation,
			&cacheRequests,
			&pb.EstCostUSD,
			&pb.FallbackRate,
		); err != nil {
			return nil, err
		}
		pb.CacheMetrics = newCacheMetrics(cacheRead, cacheCreation, cacheRequests)
		result = append(result, pb)
	}
	return result, rows.Err()
}

// DailyTokenPoint is a single day in the token trend.
type DailyTokenPoint struct {
	Date         string `json:"date"` // YYYY-MM-DD
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CacheMetrics
}

// GetDailyTokenTrend returns daily token/request aggregates for the last N days.
func (a *Analytics) GetDailyTokenTrend(days int) ([]DailyTokenPoint, error) {
	return a.GetDailyTokenTrendSince(sinceForDays(days))
}

// GetDailyTokenTrendSince returns daily aggregates from the given start time.
func (a *Analytics) GetDailyTokenTrendSince(since time.Time) ([]DailyTokenPoint, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := a.db.DB().QueryContext(ctx, `
		SELECT 
			substr(start_time, 1, 10) AS day,
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_read_input_tokens, 0) ELSE 0 END), 0) AS cache_read_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN COALESCE(cache_creation_input_tokens, 0) ELSE 0 END), 0) AS cache_creation_tokens,
			COALESCE(SUM(CASE WHEN cache_usage_reported = 1 THEN 1 ELSE 0 END), 0) AS cache_usage_requests
		FROM requests
		WHERE datetime(start_time) >= datetime(?)
		GROUP BY substr(start_time, 1, 10)
		ORDER BY day ASC
	`, since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []DailyTokenPoint
	for rows.Next() {
		var p DailyTokenPoint
		var cacheRead, cacheCreation, cacheRequests int64
		if err := rows.Scan(&p.Date, &p.Requests, &p.InputTokens, &p.OutputTokens, &cacheRead, &cacheCreation, &cacheRequests); err != nil {
			return nil, err
		}
		p.CacheMetrics = newCacheMetrics(cacheRead, cacheCreation, cacheRequests)
		result = append(result, p)
	}
	return result, rows.Err()
}
