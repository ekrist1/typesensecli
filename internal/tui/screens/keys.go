package screens

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

// Keys is the API Keys screen. It wraps the generic Resource CRUD engine but
// (1) disables editing — Typesense API keys are immutable — and (2) captures the
// create response so the generated secret (returned only once) can be shown in a
// dismissible modal.
type Keys struct {
	inner     Resource
	generated *components.JSONView // non-nil while the generated-secret modal is shown
}

const keysSaveTag = "API Keys:save"

func NewKeys(c *client.Client, w, h int) Keys {
	inner := NewResource(c, ResourceStrategy{
		TabName:       "API Keys",
		Template:      templates.APIKey,
		List:          client.ListKeys,
		Create:        client.CreateKey,
		Update:        nil, // editing disabled; the wrapper swallows the "e" key
		Delete:        client.DeleteKey,
		ExtractItems:  keysExtractItems,
		ExtractDetail: keysExtractDetail,
	}, w, h)
	return Keys{inner: inner}
}

func (s *Keys) SetSize(w, h int) {
	s.inner.SetSize(w, h)
	if s.generated != nil {
		s.generated.SetSize(w-4, h-6)
	}
}

func (s Keys) Init() tea.Cmd { return s.inner.Init() }

// HasModal reports the generated-secret modal or any inner Resource modal.
func (s Keys) HasModal() bool { return s.generated != nil || s.inner.HasModal() }

func (s Keys) Update(msg tea.Msg) (Keys, tea.Cmd) {
	// Generated-secret modal open — swallow keys until dismissed.
	if s.generated != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "enter", "q":
				s.generated = nil
				return s, nil
			case "pgup":
				s.generated.PageUp()
				return s, nil
			case "pgdown":
				s.generated.PageDown()
				return s, nil
			}
		}
		return s, nil
	}

	// Capture the secret from a successful create before the inner resource
	// consumes the message (it would otherwise just refresh the list).
	if m, ok := msg.(api.SuccessMsg); ok && m.Tag == keysSaveTag {
		view := components.NewJSONView(s.inner.width-4, s.inner.height-6)
		view.SetContent(m.Body)
		s.generated = &view
		// Still let inner refresh its list.
		var cmd tea.Cmd
		s.inner, cmd = s.inner.Update(msg)
		return s, cmd
	}

	// Disable edit: swallow "e" unless an inner modal owns the key.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "e" && !s.inner.HasModal() {
		return s, nil
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s Keys) View() string {
	if s.generated != nil {
		return "Generated API key (copy it now — it will not be shown again)\n\n" +
			s.generated.View() +
			"\n\nEsc/Enter dismiss · PgUp/PgDn scroll"
	}
	return s.inner.View()
}

// keysExtractItems parses the wrapped {"keys":[...]} list (also tolerates a bare
// array) and returns string ids derived from the numeric "id" field.
func keysExtractItems(body []byte) ([]string, error) {
	keys, err := decodeKeys(body)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, keyID(k))
	}
	return ids, nil
}

func keysExtractDetail(body []byte, id string) []byte {
	keys, err := decodeKeys(body)
	if err != nil {
		return nil
	}
	for _, k := range keys {
		if keyID(k) == id {
			b, _ := json.MarshalIndent(k, "", "  ")
			return b
		}
	}
	return nil
}

func decodeKeys(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj.Keys, nil
}

// keyID renders the numeric "id" (decoded as float64 by encoding/json) as a
// string so it can be used as a list id and in the /keys/:id path.
func keyID(k map[string]any) string {
	switch v := k["id"].(type) {
	case float64:
		return fmt.Sprintf("%d", int64(v))
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
