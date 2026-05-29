package theme

import (
	"strings"
	"testing"
)

func TestKeyHintsContainsKeysAndLabels(t *testing.T) {
	out := KeyHints([2]string{"s", "search"}, [2]string{"d", "delete"})
	for _, want := range []string{"s", "search", "d", "delete", "·"} {
		if !strings.Contains(out, want) {
			t.Fatalf("KeyHints output missing %q: %q", want, out)
		}
	}
}

func TestStatusColorsDiffer(t *testing.T) {
	ok := Status("ok", "done")
	err := Status("err", "done")
	if ok == err {
		t.Fatal("ok and err statuses render identically")
	}
	if !strings.Contains(ok, "done") || !strings.Contains(err, "done") {
		t.Fatal("status text not preserved")
	}
}
