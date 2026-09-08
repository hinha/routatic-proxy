package transformer

import (
	"encoding/json"
	"strings"
)

// normalizeToolArguments repairs insignificant whitespace around JSON object
// keys in provider-generated tool arguments. Some providers occasionally emit
// keys such as "description " instead of "description", which makes Claude
// Code reject an otherwise valid tool call during schema validation.
//
// Invalid JSON is returned unchanged so the proxy does not hide a malformed
// tool call behind a different error.
func normalizeToolArguments(raw string) string {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}

	normalized := trimJSONObjectKeys(value)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return raw
	}
	return string(encoded)
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
