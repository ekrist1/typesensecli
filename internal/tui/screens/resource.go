package screens

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clisense/internal/client"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
	"clisense/internal/tui/theme"
)

// ResourceStrategy configures how a CRUD resource screen talks to the API.
type ResourceStrategy struct {
	TabName  string
	Template []byte

	List   func() (string, string)
	Create func() (string, string) // may be nil when CreateNamePrompt is true
	Update func(id string) (string, string)
	Delete func(id string) (string, string)

	// When true, `n` first asks for a name, then calls Update(name) on submit.
	CreateNamePrompt bool

	// FooterHints overrides the default footer key hints. Each entry is
	// [2]string{key, label}. When nil, a default CRUD hint set is used.
	FooterHints [][2]string

	// ExtractItems parses the list response and returns display IDs.
	ExtractItems func(body []byte) ([]string, error)
	// ExtractDetail selects the JSON payload for a single ID from the list body.
	// Most Typesense list endpoints embed full records, so no per-item GET is needed.
	// Returns nil if not found.
	ExtractDetail func(body []byte, id string) []byte
}

type Resource struct {
	c       *client.Client
	strat   ResourceStrategy
	list    list.Model
	detail  components.JSONView
	editor  *components.JSONEditor
	confirm *components.Confirm

	namePrompt  *textinput.Model // open when user is naming a new upsert-by-name item
	pendingName string           // name captured by namePrompt, used on editor submit
	selectID    string           // item to reselect after a refresh/save

	rawList               []byte
	listW, detailW, paneH int
	status                string
	width, height         int
	isEditing             bool
	saveInFlight          bool
}

type resourceItem struct{ id string }

func (r resourceItem) Title() string       { return r.id }
func (r resourceItem) Description() string { return "" }
func (r resourceItem) FilterValue() string { return r.id }

func NewResource(c *client.Client, s ResourceStrategy, w, h int) Resource {
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), max(1, listWidth-2), max(1, paneHeight-2))
	l.Title = s.TabName
	l.SetShowTitle(false) // the panel border shows the title instead
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	detail := components.NewJSONView(max(1, detailWidth-2), max(1, paneHeight-2))
	detail.SetContent([]byte("No item selected."))
	return Resource{
		c:      c,
		strat:  s,
		list:   l,
		detail: detail,
		width:  w, height: h,
		listW: listWidth, detailW: detailWidth, paneH: paneHeight,
	}
}

func (r *Resource) SetSize(w, h int) {
	r.width, r.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	r.listW, r.detailW, r.paneH = listWidth, detailWidth, paneHeight
	r.list.SetSize(max(1, listWidth-2), max(1, paneHeight-2))
	r.detail.SetSize(max(1, detailWidth-2), max(1, paneHeight-2))
	if r.editor != nil {
		r.editor.SetSize(w, h)
	}
}

func (r Resource) tag(op string) string { return r.strat.TabName + ":" + op }

// HasModal reports whether a name prompt, JSON editor, or confirm dialog is
// currently displayed. The host app uses this to gate global shortcut keys.
func (r Resource) HasModal() bool {
	return r.list.SettingFilter() || r.namePrompt != nil || r.editor != nil || r.confirm != nil
}

func (r Resource) Init() tea.Cmd { return r.fetchList() }

func (r Resource) fetchList() tea.Cmd {
	m, p := r.strat.List()
	return api.DoRequest(r.c, r.tag("list"), m, p, nil)
}

func (r Resource) Update(msg tea.Msg) (Resource, tea.Cmd) {
	switch m := msg.(type) {
	case api.SuccessMsg:
		if m.Tag == r.tag("save") && r.editor != nil {
			r.editor = nil
			r.pendingName = ""
			r.isEditing = false
			r.saveInFlight = false
			r.status = "OK"
			return r, r.fetchList()
		}
	case api.ErrorMsg:
		if m.Tag == r.tag("save") && r.editor != nil {
			r.saveInFlight = false
			r.editor.SetError(formatAPIError(m))
			return r, nil
		}
	}

	// Name prompt modal — open when creating an upsert-by-name resource.
	if r.namePrompt != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				r.namePrompt = nil
				return r, nil
			case "enter":
				name := r.namePrompt.Value()
				if name == "" {
					return r, nil
				}
				r.pendingName = name
				r.namePrompt = nil
				ed := components.NewJSONEditor("New "+r.strat.TabName+": "+name, r.strat.Template, r.width, r.height)
				r.editor = &ed
				r.isEditing = false
				return r, nil
			}
		}
		updated, cmd := r.namePrompt.Update(msg)
		r.namePrompt = &updated
		return r, cmd
	}

	// Editor modal.
	if r.editor != nil {
		cmd := r.editor.Update(msg)
		switch m := msg.(type) {
		case components.JSONEditorCancelMsg:
			r.editor = nil
			r.pendingName = ""
			r.isEditing = false
			r.saveInFlight = false
			return r, nil
		case components.JSONEditorSubmitMsg:
			if r.saveInFlight {
				r.editor.SetError("save already in progress")
				return r, nil
			}
			var method, path string
			switch {
			case r.pendingName != "":
				method, path = r.strat.Update(r.pendingName)
				r.selectID = r.pendingName
			case r.isEditing:
				if it, ok := r.list.SelectedItem().(resourceItem); ok {
					method, path = r.strat.Update(it.id)
					r.selectID = it.id
				}
			default:
				if r.strat.Create == nil {
					r.status = "create not supported (missing name)"
					r.editor = nil
					return r, nil
				}
				method, path = r.strat.Create()
				r.selectID = ""
			}
			r.saveInFlight = true
			r.editor.SetError("saving...")
			return r, api.DoRequest(r.c, r.tag("save"), method, path, m.Body)
		}
		return r, cmd
	}

	// Confirm modal.
	if r.confirm != nil {
		cmd := r.confirm.Update(msg)
		if res, ok := msg.(components.ConfirmResultMsg); ok && res.Tag == r.tag("delete") {
			r.confirm = nil
			if res.Confirmed {
				if it, ok := r.list.SelectedItem().(resourceItem); ok {
					m, p := r.strat.Delete(it.id)
					return r, api.DoRequest(r.c, r.tag("delete"), m, p, nil)
				}
			}
			return r, nil
		}
		return r, cmd
	}

	switch m := msg.(type) {
	case api.SuccessMsg:
		switch m.Tag {
		case r.tag("list"):
			selectedID := r.selectID
			if selectedID == "" {
				if it, ok := r.list.SelectedItem().(resourceItem); ok {
					selectedID = it.id
				}
			}
			r.rawList = m.Body
			ids, err := r.strat.ExtractItems(m.Body)
			if err != nil {
				r.status = "parse error: " + err.Error()
				return r, nil
			}
			items := make([]list.Item, 0, len(ids))
			for _, id := range ids {
				items = append(items, resourceItem{id: id})
			}
			r.list.SetItems(items)
			r.status = fmt.Sprintf("%d items", len(items))
			r.selectID = ""
			if len(ids) == 0 {
				r.detail.SetContent([]byte("No items found."))
				return r, nil
			}
			selectedIndex := 0
			for i, id := range ids {
				if id == selectedID {
					selectedIndex = i
					break
				}
			}
			r.list.Select(selectedIndex)
			if d := r.strat.ExtractDetail(r.rawList, ids[selectedIndex]); d != nil {
				r.detail.SetContent(d)
			} else {
				r.detail.SetContent([]byte("No detail available for the selected item."))
			}
			return r, nil
		case r.tag("save"):
			r.editor = nil
			r.pendingName = ""
			r.isEditing = false
			r.saveInFlight = false
			r.status = "OK"
			return r, r.fetchList()
		case r.tag("delete"):
			r.status = "OK"
			return r, r.fetchList()
		}
	case api.ErrorMsg:
		if m.Tag == r.tag("save") && r.editor != nil {
			r.saveInFlight = false
			r.editor.SetError(formatAPIError(m))
			return r, nil
		}
		if m.Err != nil {
			r.status = "network error: " + m.Err.Error()
		} else {
			r.status = fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
		}
		return r, nil
	case tea.KeyMsg:
		if !r.list.SettingFilter() {
			switch m.String() {
			case "pgup":
				r.detail.PageUp()
				return r, nil
			case "pgdown":
				r.detail.PageDown()
				return r, nil
			}
		}
		switch m.String() {
		case "r":
			return r, r.fetchList()
		case "n":
			if r.strat.CreateNamePrompt {
				ti := textinput.New()
				ti.Placeholder = "name"
				ti.Focus()
				r.namePrompt = &ti
				return r, textinput.Blink
			}
			ed := components.NewJSONEditor("New "+r.strat.TabName, r.strat.Template, r.width, r.height)
			r.editor = &ed
			r.isEditing = false
			return r, nil
		case "e":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				body := r.strat.ExtractDetail(r.rawList, it.id)
				if body == nil {
					body = r.strat.Template
				}
				ed := components.NewJSONEditor("Edit "+it.id, body, r.width, r.height)
				r.editor = &ed
				r.isEditing = true
			}
			return r, nil
		case "d":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				cf := components.NewConfirm("Delete "+it.id+"?", r.tag("delete"))
				r.confirm = &cf
			}
			return r, nil
		case "enter":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				if d := r.strat.ExtractDetail(r.rawList, it.id); d != nil {
					r.detail.SetContent(d)
				}
			}
			return r, nil
		}
	}

	var cmd tea.Cmd
	r.list, cmd = r.list.Update(msg)
	if it, ok := r.list.SelectedItem().(resourceItem); ok {
		if d := r.strat.ExtractDetail(r.rawList, it.id); d != nil {
			r.detail.SetContent(d)
		}
	}
	return r, cmd
}

func (r Resource) View() string {
	if r.namePrompt != nil {
		return "Name for new " + r.strat.TabName + ":\n\n" + r.namePrompt.View() + "\n\nEnter confirm · Esc cancel"
	}
	if r.editor != nil {
		return r.editor.View()
	}
	if r.confirm != nil {
		return r.confirm.View()
	}
	hints := r.strat.FooterHints
	if hints == nil {
		hints = [][2]string{
			{"/", "filter"}, {"n", "new"}, {"e", "edit"}, {"d", "delete"},
			{"PgUp/PgDn", "scroll"}, {"r", "refresh"}, {"q", "quit"},
		}
	}
	footer := theme.KeyHints(hints...)
	if r.status != "" {
		footer = theme.Status(statusKind(r.status), r.status) + theme.StatusBar.Render(" · ") + footer
	}
	listPanel := theme.Panel{Title: r.strat.TabName, Width: r.listW, Height: r.paneH, Focused: true}
	detailPanel := theme.Panel{Title: "Detail", Width: r.detailW, Height: r.paneH}
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel.Render(r.list.View()), " ", detailPanel.Render(r.detail.View()))
	return body + "\n" + footer
}

func formatAPIError(m api.ErrorMsg) string {
	if m.Err != nil {
		return "network error: " + m.Err.Error()
	}
	return fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
}
