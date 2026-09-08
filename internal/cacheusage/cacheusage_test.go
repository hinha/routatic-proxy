package cacheusage

import "testing"

func TestUsageRateUsesProviderReportedTokenTotals(t *testing.T) {
	t.Parallel()

	usage := Usage{ReadTokens: 75, CreationTokens: 25, Reported: true}
	got, ok := usage.Rate()
	if !ok {
		t.Fatal("Rate() reported unavailable usage")
	}
	if want := 0.75; got != want {
		t.Fatalf("Rate() = %v, want %v", got, want)
	}
}

func TestUsageRateExcludesUnreportedUsage(t *testing.T) {
	t.Parallel()

	for _, usage := range []Usage{
		{ReadTokens: 75, CreationTokens: 25},
		{ReadTokens: 0, CreationTokens: 0, Reported: true},
	} {
		if got, ok := usage.Rate(); ok {
			t.Fatalf("Rate() = %v, want unavailable", got)
		}
	}
}
