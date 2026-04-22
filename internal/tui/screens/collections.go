package screens

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

const tagCollections = "collections"
const tagCollectionDetail = "collection-detail"

type collectionItem struct{ name string }

func (c collectionItem) Title() string       { return c.name }
func (c collectionItem) Description() string { return "" }
func (c collectionItem) FilterValue() string { return c.name }

type Collections struct {
	c       *client.Client
	list    list.Model
	detail  components.JSONView
	width   int
	height  int
	status  string
}

func NewCollections(c *client.Client, w, h int) Collections {
	l := list.New(nil, list.NewDefaultDelegate(), w/2, h-4)
	l.Title = "Collections"
	l.SetShowStatusBar(false)
	return Collections{c: c, list: l, detail: components.NewJSONView(w/2, h-4), width: w, height: h}
}

func (s Collections) Init() tea.Cmd { return s.fetchList() }

func (s *Collections) SetSize(w, h int) {
	s.width, s.height = w, h
	s.list.SetSize(w/2, h-4)
	s.detail.SetSize(w/2, h-4)
}

func (s Collections) fetchList() tea.Cmd {
	m, p := client.ListCollections()
	return api.DoRequest(s.c, tagCollections, m, p, nil)
}

func (s Collections) fetchDetail(name string) tea.Cmd {
	m, p := client.GetCollection(name)
	return api.DoRequest(s.c, tagCollectionDetail, m, p, nil)
}

func (s Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	switch m := msg.(type) {
	case api.SuccessMsg:
		switch m.Tag {
		case tagCollections:
			var arr []map[string]any
			if err := json.Unmarshal(m.Body, &arr); err != nil {
				s.status = "parse error: " + err.Error()
				return s, nil
			}
			items := make([]list.Item, 0, len(arr))
			for _, c := range arr {
				if name, ok := c["name"].(string); ok {
					items = append(items, collectionItem{name: name})
				}
			}
			s.list.SetItems(items)
			s.status = fmt.Sprintf("%d collections", len(items))
			return s, nil
		case tagCollectionDetail:
			s.detail.SetContent(m.Body)
			return s, nil
		}
	case api.ErrorMsg:
		if m.Err != nil {
			s.status = "network error: " + m.Err.Error()
		} else {
			s.status = fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
		}
		return s, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.fetchList()
		case "enter":
			if it, ok := s.list.SelectedItem().(collectionItem); ok {
				return s, s.fetchDetail(it.name)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s Collections) View() string {
	footer := "Enter view schema · r refresh · Esc back"
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	return s.list.View() + "    " + s.detail.View() + "\n" + footer
}
