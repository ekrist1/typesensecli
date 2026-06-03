package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
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

func TestKeysAppliesListRefreshWhileGeneratedSecretIsOpen(t *testing.T) {
	k := NewKeys(nil, 80, 24)
	ed := components.NewJSONEditor("New API Keys", []byte(`{"description":"x"}`), 80, 24)
	k.inner.editor = &ed

	k, cmd := k.Update(api.SuccessMsg{Tag: keysSaveTag, Status: 201, Body: []byte(`{"id":9,"value":"SECRETVALUE","description":"x"}`)})
	if cmd == nil {
		t.Fatal("expected refresh command after API key create")
	}
	if k.generated == nil {
		t.Fatal("expected generated-key modal to be open after create")
	}

	k, cmd = k.Update(api.SuccessMsg{Tag: "API Keys:list", Status: 200, Body: []byte(`{"keys":[{"id":9,"description":"x","value_prefix":"abc"}]}`)})
	if cmd != nil {
		cmd()
	}
	items := k.inner.list.Items()
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item, ok := items[0].(resourceItem)
	if !ok {
		t.Fatalf("item type = %T, want resourceItem", items[0])
	}
	if item.id != "9" {
		t.Fatalf("item id = %q, want 9", item.id)
	}
	if k.generated == nil {
		t.Fatal("generated-key modal closed while applying list refresh")
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
