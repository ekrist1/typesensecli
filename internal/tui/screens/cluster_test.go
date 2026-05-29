package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

func TestClusterStoresPanelPayload(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	c, _ = c.Update(api.SuccessMsg{Tag: "Cluster:health", Status: 200, Body: []byte(`{"ok":true}`)})
	if got := string(c.payloads["Cluster:health"]); got != `{"ok":true}` {
		t.Fatalf("payload not stored, got %q", got)
	}
}

func TestClusterOperationOpensConfirm(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	// "C" requests DB compaction and must open a confirm dialog, not fire immediately.
	c, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if !c.HasModal() {
		t.Fatal("expected confirm dialog to open for compact operation")
	}
	if cmd != nil {
		t.Fatal("operation must not fire before confirmation")
	}
}

func TestClusterConfirmedOperationFires(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")}) // clear cache
	if !c.HasModal() {
		t.Fatal("expected confirm dialog")
	}
	_, cmd := c.Update(components.ConfirmResultMsg{Tag: "Cluster:op:clear-cache", Confirmed: true})
	if cmd == nil {
		t.Fatal("expected the clear-cache request command after confirmation")
	}
}
