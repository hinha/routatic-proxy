package handlers

import (
	"testing"

	"github.com/routatic/proxy/internal/cacheusage"
	"github.com/routatic/proxy/pkg/types"
)

func TestResponseWriterExtractsReportedCacheUsage(t *testing.T) {
	t.Parallel()

	rw := &responseWriter{}
	rw.extractUsageFromSSE([]byte(`event: message_delta
data: {"type":"message_delta","usage":{"input_tokens":20,"output_tokens":5,"cache_read_input_tokens":75,"cache_creation_input_tokens":25}}

`))

	got := rw.cacheUsage()
	want := cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true}
	if got != want {
		t.Fatalf("cacheUsage() = %+v, want %+v", got, want)
	}
}

func TestCacheUsageFromResponseRequiresUsageFields(t *testing.T) {
	t.Parallel()

	response := types.MessageResponse{Usage: types.Usage{
		InputTokens:              20,
		OutputTokens:             5,
		CacheReadInputTokens:     75,
		CacheCreationInputTokens: 25,
	}}
	body := []byte(`{"content":[{"text":"the key cache_read_input_tokens is not telemetry"}],"usage":{"input_tokens":20,"output_tokens":5,"cache_read_input_tokens":75,"cache_creation_input_tokens":25}}`)

	got := cacheUsageFromResponse(body, response)
	want := cacheusage.Usage{ReadTokens: 75, CreationTokens: 25, Reported: true}
	if got != want {
		t.Fatalf("cacheUsageFromResponse() = %+v, want %+v", got, want)
	}

	withoutUsage := []byte(`{"content":[{"text":"cache_read_input_tokens"}]}`)
	if got := cacheUsageFromResponse(withoutUsage, response); got.Reported {
		t.Fatalf("cacheUsageFromResponse() = %+v, want unreported usage", got)
	}
}

func TestResponseWriterDoesNotInferMissingCacheUsage(t *testing.T) {
	t.Parallel()

	rw := &responseWriter{}
	rw.extractUsageFromSSE([]byte(`event: message_delta
data: {"type":"message_delta","usage":{"input_tokens":20,"output_tokens":5}}

`))

	if got := rw.cacheUsage(); got.Reported {
		t.Fatalf("cacheUsage() = %+v, want unreported usage", got)
	}
}
