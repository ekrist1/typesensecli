package screens

import "testing"

func TestStatusKind(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"OK":                 "ok",
		"5 items":            "",
		"parse error: bad":   "err",
		"network error: x":   "err",
		"HTTP 422: bad body": "err",
	}
	for in, want := range cases {
		if got := statusKind(in); got != want {
			t.Fatalf("statusKind(%q) = %q, want %q", in, got, want)
		}
	}
}
