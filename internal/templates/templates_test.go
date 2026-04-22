package templates

import (
	"encoding/json"
	"testing"
)

func TestAllTemplatesValidJSON(t *testing.T) {
	for name, body := range All() {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Errorf("template %q invalid: %v", name, err)
		}
	}
}
