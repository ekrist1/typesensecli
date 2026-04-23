package screens

import (
	"encoding/json"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui/api"
)

// Curations wraps the generic Resource screen for curation sets. It adds two
// create paths:
//
//	n → guided wizard (newCurationWizard) for a single-item set
//	N → raw JSON editor (existing Resource name-prompt → editor path)
//
// The wizard is a prototype: it supports one item per set with one of three
// rule types (query+match, filter_by, tags). Multi-item sets or optional
// overrides (sort_by, replace_query, etc.) go through the raw path.
type Curations struct {
	inner      Resource
	wizardName *textinput.Model
	wizard     *curationWizard
}

func NewCurations(c *client.Client, w, h int) Curations {
	inner := NewResource(c, ResourceStrategy{
		TabName:          "Curations",
		Template:         templates.Curation,
		List:             client.ListCurationSets,
		Create:           nil,
		Update:           client.UpsertCurationSet,
		Delete:           client.DeleteCurationSet,
		CreateNamePrompt: true,
		ExtractItems:     curationsExtractItems,
		ExtractDetail:    curationsExtractDetail,
	}, w, h)
	return Curations{inner: inner}
}

func (s *Curations) SetSize(w, h int) { s.inner.SetSize(w, h) }

func (s Curations) Init() tea.Cmd { return s.inner.Init() }

func (s Curations) Update(msg tea.Msg) (Curations, tea.Cmd) {
	// Wizard name prompt: capture the set name, then open the wizard.
	if s.wizardName != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				s.wizardName = nil
				return s, nil
			case "enter":
				name := s.wizardName.Value()
				if name == "" {
					return s, nil
				}
				s.wizardName = nil
				wiz := newCurationWizard(name)
				s.wizard = &wiz
				return s, textinput.Blink
			}
		}
		updated, cmd := s.wizardName.Update(msg)
		s.wizardName = &updated
		return s, cmd
	}

	// Wizard active: route all input to it, then check for submit/cancel.
	if s.wizard != nil {
		updated, cmd := s.wizard.Update(msg)
		s.wizard = &updated
		if body := s.wizard.Submitted(); body != nil {
			name := s.wizard.Name()
			s.wizard = nil
			m, p := client.UpsertCurationSet(name)
			return s, api.DoRequest(s.inner.c, s.inner.tag("save"), m, p, body)
		}
		if s.wizard.Cancelled() {
			s.wizard = nil
		}
		return s, cmd
	}

	// Intercept `n` and `N` only when no inner modal is open.
	if km, ok := msg.(tea.KeyMsg); ok && s.inner.namePrompt == nil && s.inner.editor == nil && s.inner.confirm == nil {
		switch km.String() {
		case "n":
			ti := textinput.New()
			ti.Placeholder = "curation set name"
			ti.Focus()
			s.wizardName = &ti
			return s, textinput.Blink
		case "N":
			// Raw JSON path: mirror Resource's own `n` handler for
			// CreateNamePrompt resources.
			ti := textinput.New()
			ti.Placeholder = "curation set name"
			ti.Focus()
			s.inner.namePrompt = &ti
			return s, textinput.Blink
		}
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s Curations) View() string {
	if s.wizardName != nil {
		return "Curation set name (wizard):\n\n" + s.wizardName.View() + "\n\nEnter continue · Esc cancel"
	}
	if s.wizard != nil {
		return s.wizard.View()
	}
	return s.inner.View()
}

func curationsExtractItems(body []byte) ([]string, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(arr))
	for _, m := range arr {
		if name, ok := m["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

func curationsExtractDetail(body []byte, name string) []byte {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil
	}
	for _, m := range arr {
		if m["name"] == name {
			b, _ := json.MarshalIndent(m, "", "  ")
			return b
		}
	}
	return nil
}
