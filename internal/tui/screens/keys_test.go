package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
)

func TestKeysExtractItemsWrappedAndNumericID(t *testing.T) {
	body := []byte(`{"keys":[{"id":0,"description":"admin","value_prefix":"abc"},{"id":7,"description":"search","value_prefix":"xyz"}]}`)
	ids, err := keysExtractItems(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "0" || ids[1] != "7" {
		t.Fatalf("got %v, want [0 7]", ids)
	}
}

func TestKeysCapturesGeneratedSecretOnCreate(t *testing.T) {
	k := NewKeys(nil, 80, 24)
	// Simulate a successful create response carrying the full secret.
	saveTag := "API Keys:save"
	k, _ = k.Update(api.SuccessMsg{Tag: saveTag, Status: 201, Body: []byte(`{"id":9,"value":"SECRETVALUE","description":"x"}`)})
	if !k.HasModal() {
		t.Fatal("expected generated-key modal to be open after create")
	}
	if k.generated == nil {
		t.Fatal("generated key payload not captured")
	}
	// Dismissing with esc closes the modal.
	k, _ = k.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if k.generated != nil {
		t.Fatal("generated-key modal still open after esc")
	}
}

func TestKeysSwallowsEditKey(t *testing.T) {
	k := NewKeys(nil, 80, 24)
	// 'e' must not open an editor on the inner resource (keys are immutable).
	k, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if k.inner.editor != nil {
		t.Fatal("edit must be disabled for API keys")
	}
}
