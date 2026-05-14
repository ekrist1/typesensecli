package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clisense/internal/client"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

const tagCollections = "collections"
const tagCollectionDetail = "collection-detail"
const tagCollectionDelete = "collection-delete"

type collectionItem struct{ name string }

func (c collectionItem) Title() string       { return c.name }
func (c collectionItem) Description() string { return "" }
func (c collectionItem) FilterValue() string { return c.name }

type collectionFocus int

const (
	focusCollectionList collectionFocus = iota
	focusFieldTable
)

type collectionSchemaPayload struct {
	Name                string           `json:"name"`
	CreatedAt           int64            `json:"created_at"`
	CurationSets        []string         `json:"curation_sets"`
	SynonymSets         []string         `json:"synonym_sets"`
	DefaultSortingField string           `json:"default_sorting_field"`
	EnableNestedFields  bool             `json:"enable_nested_fields"`
	Fields              []map[string]any `json:"fields"`
}

type collectionFieldItem struct {
	name   string
	typ    string
	flags  string
	detail []byte
}

type collectionSchemaView struct {
	name           string
	summary        string
	fields         []collectionFieldItem
	searchableKeys []string
}

type Collections struct {
	c      *client.Client
	list   list.Model
	fields table.Model
	detail components.JSONView
	width  int
	height int
	status string

	focus             collectionFocus
	loadedCollection  string
	pendingCollection string
	schema            collectionSchemaView
	search            *collectionSearchModal
	confirm           *components.Confirm
}

func NewCollections(c *client.Client, w, h int) Collections {
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), listWidth, paneHeight)
	l.Title = "Collections *"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	tbl := newCollectionFieldTable(fieldsWidth, paneHeight)
	detail := components.NewJSONView(detailWidth, paneHeight)
	detail.SetContent([]byte("Select a collection to inspect its schema fields."))

	s := Collections{
		c:      c,
		list:   l,
		fields: tbl,
		detail: detail,
		width:  w,
		height: h,
		focus:  focusCollectionList,
	}
	s.syncFocus()
	return s
}

func (s Collections) Init() tea.Cmd { return s.fetchList() }

func (s Collections) HasModal() bool {
	return s.list.SettingFilter() || s.search != nil || s.confirm != nil
}

func (s *Collections) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	s.list.SetSize(listWidth, paneHeight)
	s.fields.SetColumns(collectionFieldColumns(fieldsWidth))
	s.fields.SetWidth(fieldsWidth)
	s.fields.SetHeight(paneHeight)
	s.detail.SetSize(detailWidth, paneHeight)
	if s.search != nil {
		s.search.SetSize(w, h)
	}
}

func (s Collections) fetchList() tea.Cmd {
	m, p := client.ListCollections()
	return api.DoRequest(s.c, tagCollections, m, p, nil)
}

func (s Collections) fetchDetail(name string) tea.Cmd {
	m, p := client.GetCollection(name)
	return api.DoRequest(s.c, collectionDetailTag(name), m, p, nil)
}

func (s *Collections) loadCollection(name string) tea.Cmd {
	if name == "" || name == s.pendingCollection || name == s.loadedCollection {
		return nil
	}
	s.pendingCollection = name
	s.status = "loading " + name
	return s.fetchDetail(name)
}

func (s *Collections) clearSchema(message string) {
	s.schema = collectionSchemaView{}
	s.loadedCollection = ""
	s.pendingCollection = ""
	s.fields.SetRows(nil)
	s.detail.SetContent([]byte(message))
}

func (s *Collections) applySchema(schema collectionSchemaView) {
	s.schema = schema
	rows := make([]table.Row, 0, len(schema.fields)+1)
	rows = append(rows, table.Row{"@schema", "collection", fmt.Sprintf("%d fields", len(schema.fields))})
	for _, field := range schema.fields {
		rows = append(rows, table.Row{field.name, field.typ, field.flags})
	}
	s.fields.SetRows(rows)
	s.fields.SetCursor(0)
	s.syncFieldDetail()
}

func (s *Collections) syncFocus() {
	if s.focus == focusCollectionList {
		s.list.Title = "Collections *"
		s.fields.Blur()
		s.fields.SetStyles(collectionFieldTableStyles(false))
		return
	}
	s.list.Title = "Collections"
	s.fields.Focus()
	s.fields.SetStyles(collectionFieldTableStyles(true))
}

func (s *Collections) syncFieldDetail() {
	if len(s.fields.Rows()) == 0 {
		s.detail.SetContent([]byte("Select a collection to inspect its schema fields."))
		return
	}
	if s.fields.Cursor() == 0 {
		s.detail.SetContent([]byte(s.schema.summary))
		return
	}

	fieldIndex := s.fields.Cursor() - 1
	if fieldIndex >= 0 && fieldIndex < len(s.schema.fields) {
		s.detail.SetContent(s.schema.fields[fieldIndex].detail)
	}
}

func (s Collections) selectedCollectionName() string {
	if it, ok := s.list.SelectedItem().(collectionItem); ok {
		return it.name
	}
	return ""
}

func (s Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	if s.search != nil {
		var cmd tea.Cmd
		updated, cmd := s.search.Update(msg)
		s.search = &updated
		if s.search.Closed() {
			s.search = nil
			return s, nil
		}
		return s, cmd
	}
	if s.confirm != nil {
		cmd := s.confirm.Update(msg)
		if res, ok := msg.(components.ConfirmResultMsg); ok && res.Tag == tagCollectionDelete {
			s.confirm = nil
			if res.Confirmed {
				collection := s.selectedCollectionName()
				if collection == "" {
					s.status = "pick a collection first"
					return s, nil
				}
				s.loadedCollection = ""
				s.pendingCollection = ""
				m, p := client.DeleteCollection(collection)
				return s, api.DoRequest(s.c, tagCollectionDelete, m, p, nil)
			}
			return s, nil
		}
		return s, cmd
	}

	switch m := msg.(type) {
	case api.SuccessMsg:
		switch {
		case m.Tag == tagCollections:
			selectedName := s.selectedCollectionName()
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
			if len(items) == 0 {
				s.status = "0 collections"
				s.clearSchema("No collections found.")
				return s, nil
			}

			selectedIndex := 0
			for i, item := range items {
				if it, ok := item.(collectionItem); ok && it.name == selectedName {
					selectedIndex = i
					break
				}
			}
			s.list.Select(selectedIndex)
			s.status = fmt.Sprintf("%d collections", len(items))
			return s, s.loadCollection(s.selectedCollectionName())

		case strings.HasPrefix(m.Tag, tagCollectionDetail+":"):
			name := strings.TrimPrefix(m.Tag, tagCollectionDetail+":")
			if name != s.selectedCollectionName() {
				return s, nil
			}

			s.pendingCollection = ""
			s.loadedCollection = name
			s.status = "loaded " + name

			schema, err := buildCollectionSchemaView(m.Body)
			if err != nil {
				s.status = "parse error: " + err.Error()
				s.clearSchema("Unable to parse collection schema.")
				return s, nil
			}
			s.applySchema(schema)
			return s, nil
		case m.Tag == tagCollectionDelete:
			s.status = "deleted collection"
			return s, s.fetchList()
		}

	case api.ErrorMsg:
		if m.Tag == tagCollectionDelete {
			s.pendingCollection = ""
			s.loadedCollection = ""
		}
		if strings.HasPrefix(m.Tag, tagCollectionDetail+":") {
			name := strings.TrimPrefix(m.Tag, tagCollectionDetail+":")
			if name != s.selectedCollectionName() {
				return s, nil
			}
			s.pendingCollection = ""
		}
		if m.Err != nil {
			s.status = "network error: " + m.Err.Error()
		} else {
			s.status = fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
		}
		return s, nil

	case tea.KeyMsg:
		if !s.list.SettingFilter() {
			switch m.String() {
			case "s":
				collection := s.selectedCollectionName()
				if collection == "" {
					s.status = "pick a collection first"
					return s, nil
				}
				searchable := s.schema.searchableKeys
				if s.loadedCollection != collection {
					searchable = nil
				}
				modal := newCollectionSearchModal(s.c, collection, searchable, s.width, s.height)
				s.search = &modal
				return s, textinput.Blink
			case "right", "l":
				if len(s.fields.Rows()) > 0 {
					s.focus = focusFieldTable
					s.syncFocus()
					return s, nil
				}
			case "left", "h":
				s.focus = focusCollectionList
				s.syncFocus()
				return s, nil
			case "ctrl+b":
				s.detail.PageUp()
				return s, nil
			case "ctrl+f":
				s.detail.PageDown()
				return s, nil
			case "d":
				collection := s.selectedCollectionName()
				if collection == "" {
					s.status = "pick a collection first"
					return s, nil
				}
				cf := components.NewConfirm("Delete collection "+collection+"? This cannot be undone.", tagCollectionDelete)
				s.confirm = &cf
				return s, nil
			}
		}
		switch m.String() {
		case "r":
			s.status = "refreshing collections"
			s.loadedCollection = ""
			s.pendingCollection = ""
			return s, s.fetchList()
		case "enter":
			if s.focus == focusCollectionList {
				return s, s.loadCollection(s.selectedCollectionName())
			}
		}
	}

	if s.focus == focusFieldTable {
		var cmd tea.Cmd
		s.fields, cmd = s.fields.Update(msg)
		s.syncFieldDetail()
		return s, cmd
	}

	before := s.selectedCollectionName()
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	after := s.selectedCollectionName()
	if after != "" && after != before && !s.list.SettingFilter() {
		return s, s.loadCollection(after)
	}
	return s, cmd
}

func (s Collections) View() string {
	if s.search != nil {
		return s.search.View()
	}
	if s.confirm != nil {
		return s.confirm.View()
	}
	footer := "←/→ switch pane · / filter collections · Ctrl+B/Ctrl+F scroll config · r refresh · q quit"
	if s.focus == focusFieldTable {
		footer = "focus fields · " + footer
	} else {
		footer = "focus collections · " + footer
	}
	footer = "s search documents · d delete collection · " + footer
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, s.list.View(), "  ", s.fields.View(), "  ", s.detail.View())
	return body + "\n" + footer
}

func buildCollectionSchemaView(body []byte) (collectionSchemaView, error) {
	var payload collectionSchemaPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return collectionSchemaView{}, err
	}

	view := collectionSchemaView{
		name:    payload.Name,
		summary: renderCollectionSummary(payload),
		fields:  make([]collectionFieldItem, 0, len(payload.Fields)),
	}

	for _, field := range payload.Fields {
		name := stringFromMap(field, "name")
		if name == "" {
			name = "<unnamed>"
		}
		typ := stringFromMap(field, "type")
		if typ == "" {
			typ = "-"
		}
		detail, err := json.MarshalIndent(field, "", "  ")
		if err != nil {
			return collectionSchemaView{}, err
		}
		view.fields = append(view.fields, collectionFieldItem{
			name:   name,
			typ:    typ,
			flags:  collectionFieldFlags(field),
			detail: detail,
		})
	}
	view.searchableKeys = searchableCollectionFields(view.fields)

	return view, nil
}

func renderCollectionSummary(payload collectionSchemaPayload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Collection: %s\n\n", payload.Name)
	fmt.Fprintf(&b, "Fields: %d\n", len(payload.Fields))
	if payload.DefaultSortingField == "" {
		fmt.Fprintln(&b, "Default sorting field: -")
	} else {
		fmt.Fprintf(&b, "Default sorting field: %s\n", payload.DefaultSortingField)
	}
	fmt.Fprintf(&b, "Nested fields: %t\n", payload.EnableNestedFields)
	if payload.CreatedAt > 0 {
		fmt.Fprintf(&b, "Created at: %s\n", time.Unix(payload.CreatedAt, 0).UTC().Format(time.RFC3339))
	}
	if len(payload.CurationSets) == 0 {
		fmt.Fprintln(&b, "Curation sets: -")
	} else {
		fmt.Fprintf(&b, "Curation sets: %s\n", strings.Join(payload.CurationSets, ", "))
	}
	if len(payload.SynonymSets) == 0 {
		fmt.Fprintln(&b, "Synonym sets: -")
	} else {
		fmt.Fprintf(&b, "Synonym sets: %s\n", strings.Join(payload.SynonymSets, ", "))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Use Left/Right to switch between collections and fields.")
	fmt.Fprintln(&b, "Select a field in the middle pane to inspect its JSON config here.")
	return b.String()
}

func collectionFieldFlags(field map[string]any) string {
	flags := make([]string, 0, 8)
	for _, key := range []string{"index", "facet", "optional", "sort", "store", "infix", "stem"} {
		if booleanFromMap(field, key) {
			flags = append(flags, key)
		}
	}
	if locale := stringFromMap(field, "locale"); locale != "" {
		flags = append(flags, "locale="+locale)
	}
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ", ")
}

func stringFromMap(m map[string]any, key string) string {
	if value, ok := m[key].(string); ok {
		return value
	}
	return ""
}

func booleanFromMap(m map[string]any, key string) bool {
	if value, ok := m[key].(bool); ok {
		return value
	}
	return false
}

func newCollectionFieldTable(width, height int) table.Model {
	tbl := table.New(
		table.WithColumns(collectionFieldColumns(width)),
		table.WithHeight(height),
		table.WithWidth(width),
		table.WithFocused(false),
		table.WithStyles(collectionFieldTableStyles(false)),
	)
	return tbl
}

func collectionFieldColumns(width int) []table.Column {
	if width <= 0 {
		return nil
	}
	nameWidth := max(12, width/2)
	typeWidth := max(8, min(14, width/5))
	flagsWidth := max(10, width-nameWidth-typeWidth)
	return []table.Column{
		{Title: "Field", Width: nameWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Flags", Width: flagsWidth},
	}
}

func collectionFieldTableStyles(focused bool) table.Styles {
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styles.Cell = lipgloss.NewStyle()
	if focused {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	} else {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
	}
	return styles
}

func collectionPaneSizes(totalWidth, totalHeight int) (listWidth, fieldsWidth, detailWidth, paneHeight int) {
	const (
		gap                = 2
		preferredListWidth = 28
		preferredMidWidth  = 34
		minListWidth       = 22
		minMidWidth        = 24
		minDetailWidth     = 30
	)

	if totalWidth <= 0 {
		return 0, 0, 0, max(1, totalHeight-4)
	}

	usable := totalWidth - gap*2
	if usable <= 0 {
		return max(0, totalWidth), 0, 0, max(1, totalHeight-4)
	}

	if usable < minListWidth+minMidWidth+minDetailWidth {
		listWidth = max(1, usable/4)
		fieldsWidth = max(1, usable/3)
		detailWidth = max(0, usable-listWidth-fieldsWidth)
		return listWidth, fieldsWidth, detailWidth, max(1, totalHeight-4)
	}

	listWidth = min(preferredListWidth, usable/5)
	listWidth = max(minListWidth, listWidth)

	fieldsWidth = min(preferredMidWidth, usable/3)
	fieldsWidth = max(minMidWidth, fieldsWidth)

	detailWidth = usable - listWidth - fieldsWidth
	if detailWidth < minDetailWidth {
		deficit := minDetailWidth - detailWidth
		if fieldsWidth-deficit >= minMidWidth {
			fieldsWidth -= deficit
		} else {
			remaining := deficit - (fieldsWidth - minMidWidth)
			fieldsWidth = minMidWidth
			listWidth = max(minListWidth, listWidth-remaining)
		}
		detailWidth = usable - listWidth - fieldsWidth
	}

	return listWidth, fieldsWidth, max(0, detailWidth), max(1, totalHeight-4)
}

func collectionDetailTag(name string) string {
	return tagCollectionDetail + ":" + name
}
