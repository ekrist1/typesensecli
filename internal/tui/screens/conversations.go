package screens

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

type Conversations struct {
	inner Resource

	testCollection *textinput.Model
	testQuery      *textarea.Model
	testResult     *components.JSONView
	focus          int // 0 = collection, 1 = query
}

func NewConversations(c *client.Client, w, h int) Conversations {
	inner := NewResource(c, ResourceStrategy{
		TabName:  "Conversations",
		Template: templates.Conversation,
		List:     client.ListConversationModels,
		Create:   client.CreateConversationModel,
		Update:   client.UpdateConversationModel,
		Delete:   client.DeleteConversationModel,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if id, ok := m["id"].(string); ok {
					ids = append(ids, id)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, id string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["id"] == id {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
	return Conversations{inner: inner}
}

func (s *Conversations) SetSize(w, h int) { s.inner.SetSize(w, h) }

func (s Conversations) Init() tea.Cmd { return s.inner.Init() }

// HasModal reports whether the conversation test modal or any inner Resource
// modal is active.
func (s Conversations) HasModal() bool {
	return s.testQuery != nil || s.inner.HasModal()
}

const tagConvTest = "conversations:test"

func (s Conversations) Update(msg tea.Msg) (Conversations, tea.Cmd) {
	// Test modal open — route all keys to it.
	if s.testQuery != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				s.testQuery, s.testCollection, s.testResult, s.focus = nil, nil, nil, 0
				return s, nil
			case "tab", "shift+tab":
				s.focus = 1 - s.focus
				if s.focus == 0 {
					s.testCollection.Focus()
					s.testQuery.Blur()
				} else {
					s.testCollection.Blur()
					s.testQuery.Focus()
				}
				return s, nil
			case "ctrl+s":
				it, ok := s.inner.list.SelectedItem().(resourceItem)
				if !ok {
					return s, nil
				}
				reqBody := map[string]any{
					"searches": []map[string]any{{
						"collection":            s.testCollection.Value(),
						"q":                     s.testQuery.Value(),
						"query_by":              "*",
						"conversation":          true,
						"conversation_model_id": it.id,
					}},
				}
				b, _ := json.Marshal(reqBody)
				m, p := client.ConversationTest()
				return s, api.DoRequest(s.inner.c, tagConvTest, m, p, b)
			}
		}
		var cmd tea.Cmd
		if s.focus == 0 {
			*s.testCollection, cmd = s.testCollection.Update(msg)
		} else {
			*s.testQuery, cmd = s.testQuery.Update(msg)
		}
		return s, cmd
	}

	// Intercept the "t" key when inner has no modal open.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "t" && s.inner.editor == nil && s.inner.confirm == nil {
		ti := textinput.New()
		ti.Placeholder = "collection name"
		ti.Focus()
		ta := textarea.New()
		ta.Placeholder = "query"
		ta.SetWidth(s.inner.width - 4)
		ta.SetHeight(4)
		rv := components.NewJSONView(s.inner.width-4, s.inner.height-12)
		s.testCollection = &ti
		s.testQuery = &ta
		s.testResult = &rv
		s.focus = 0
		return s, textinput.Blink
	}

	// Surface test response back into the modal viewer.
	switch m := msg.(type) {
	case api.SuccessMsg:
		if m.Tag == tagConvTest && s.testResult != nil {
			s.testResult.SetContent(m.Body)
			return s, nil
		}
	case api.ErrorMsg:
		if m.Tag == tagConvTest && s.testResult != nil {
			s.testResult.SetContent([]byte(fmt.Sprintf(`{"error":"HTTP %d","body":%q}`, m.Status, string(m.Body))))
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s Conversations) View() string {
	if s.testQuery != nil {
		v := "Test conversation model\n\n"
		v += "Collection: " + s.testCollection.View() + "\n\n"
		v += "Query:\n" + s.testQuery.View() + "\n\n"
		if s.testResult != nil {
			v += "Response:\n" + s.testResult.View() + "\n"
		}
		v += "\nCtrl+S send · Tab switch · Esc close"
		return v
	}
	return s.inner.View()
}
