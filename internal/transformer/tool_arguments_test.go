package transformer

import "testing"

// Only key whitespace is repaired. Numbers and string values must survive the
// round-trip exactly as the provider sent them.
func TestNormalizeToolArgumentsPreservesValues(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trims whitespace around nested keys",
			in:   `{"questions":[{"options":[{"label":"A","description ":"answer"}]}]}`,
			// Object keys are re-emitted in sorted order; only whitespace in
			// the key names is significant here.
			want: `{"questions":[{"options":[{"description":"answer","label":"A"}]}]}`,
		},
		{
			name: "large integer keeps full precision",
			in:   `{"id":1234567890123456789}`,
			want: `{"id":1234567890123456789}`,
		},
		{
			name: "number formatting is not rewritten",
			in:   `{"price":1.0,"exp":1e21}`,
			want: `{"exp":1e21,"price":1.0}`,
		},
		{
			name: "html characters and ampersands are not escaped",
			in:   `{"html":"<b>bold</b>","amp":"a&b"}`,
			want: `{"amp":"a&b","html":"<b>bold</b>"}`,
		},
		{
			name: "invalid json is returned unchanged",
			in:   `{"description ":"unterminated}`,
			want: `{"description ":"unterminated}`,
		},
		{
			name: "trailing content is returned unchanged",
			in:   `{"a":1}{"b":2}`,
			want: `{"a":1}{"b":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeToolArguments(tt.in); got != tt.want {
				t.Fatalf("normalizeToolArguments(%s) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

// An exact key wins over a whitespace-padded variant of the same key, so an
// already-valid argument is never overwritten by a malformed duplicate.
func TestNormalizeToolArgumentsKeepsExactKeyOnCollision(t *testing.T) {
	got := normalizeToolArguments(`{"description":"exact","description ":"padded"}`)
	if want := `{"description":"exact"}`; got != want {
		t.Fatalf("normalizeToolArguments() = %s, want %s", got, want)
	}
}
