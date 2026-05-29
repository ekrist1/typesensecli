package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

func TestResourceKeepsEditorOpenOnSaveError(t *testing.T) {
	r := NewResource(nil, ResourceStrategy{
		TabName:  "Things",
		Template: []byte(`{"name":"example"}`),
		List:     func() (string, string) { return "GET", "/things" },
		Update:   func(id string) (string, string) { return "PUT", "/things/" + id },
		Delete:   func(id string) (string, string) { return "DELETE", "/things/" + id },
		ExtractItems: func(body []byte) ([]string, error) {
			return nil, nil
		},
		ExtractDetail: func(body []byte, id string) []byte {
			return nil
		},
	}, 80, 24)

	ed := components.NewJSONEditor("Edit example", []byte(`{"name":"example"}`), 80, 24)
	r.editor = &ed
	r.pendingName = "example"

	var cmd tea.Cmd
	r, cmd = r.Update(components.JSONEditorSubmitMsg{Body: []byte(`{"name":"example"}`)})
	if cmd == nil {
		t.Fatal("expected save command")
	}
	if r.editor == nil {
		t.Fatal("editor closed before save response")
	}
	if !r.saveInFlight {
		t.Fatal("saveInFlight=false, want true")
	}

	r, _ = r.Update(api.ErrorMsg{Tag: r.tag("save"), Status: 422, Body: []byte(`{"message":"bad"}`)})
	if r.editor == nil {
		t.Fatal("editor closed after failed save")
	}
	if r.saveInFlight {
		t.Fatal("saveInFlight=true after failed save")
	}
	if r.pendingName != "example" {
		t.Fatalf("pendingName=%q, want retry name preserved", r.pendingName)
	}

	r, cmd = r.Update(api.SuccessMsg{Tag: r.tag("save"), Status: 200, Body: []byte(`{}`)})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if r.editor != nil {
		t.Fatal("editor still open after successful save")
	}
	if r.pendingName != "" {
		t.Fatalf("pendingName=%q, want cleared", r.pendingName)
	}
}
