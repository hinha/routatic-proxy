// Package cacheusage defines provider-reported prompt-cache usage.
package cacheusage

// Usage contains prompt-cache token counters reported by an upstream provider.
// Reported distinguishes missing telemetry from a real zero-valued counter.
type Usage struct {
	ReadTokens     int64
	CreationTokens int64
	Reported       bool
}

// Rate returns the cache-read share of reported cache tokens.
func (u Usage) Rate() (float64, bool) {
	if !u.Reported {
		return 0, false
	}

	total := u.ReadTokens + u.CreationTokens
	if total <= 0 {
		return 0, false
	}

	return float64(u.ReadTokens) / float64(total), true
}
