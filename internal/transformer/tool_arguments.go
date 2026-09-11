package transformer

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// normalizeToolArguments repairs insignificant whitespace around JSON object
// keys in provider-generated tool arguments. Some providers occasionally emit
// keys such as "description " instead of "description", which makes Claude
// Code reject an otherwise valid tool call during schema validation.
//
// Only keys are rewritten, so a value the provider already got right is
// reproduced byte-for-byte. Decoding into float64 would round integers above
// 2^53, and the default encoder escapes "<", ">" and "&" into \uXXXX.
//
// Invalid JSON is returned unchanged so the proxy does not hide a malformed
// tool call behind a different error.
func normalizeToolArguments(raw string) string {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	// Trailing content means the argument is not a single JSON value. Leave it
	// untouched rather than silently keeping only the first one.
	if _, err := decoder.Token(); err != io.EOF {
		return raw
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(trimJSONObjectKeys(value)); err != nil {
		return raw
	}
	return strings.TrimRight(buf.String(), "\n")
}

func trimJSONObjectKeys(value any) any {
	switch value := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))

		// Preserve exact keys when a trimmed key would collide with one. This
		// keeps an already-valid argument authoritative and makes the result
		// independent of map iteration order.
		for key, child := range value {
			if strings.TrimSpace(key) == key {
				out[key] = trimJSONObjectKeys(child)
			}
		}
		for key, child := range value {
			trimmed := strings.TrimSpace(key)
			if trimmed == key {
				continue
			}
			if _, exists := out[trimmed]; !exists {
				out[trimmed] = trimJSONObjectKeys(child)
			}
		}
		return out
	case []any:
		for i, child := range value {
			value[i] = trimJSONObjectKeys(child)
		}
	}
	return value
}
