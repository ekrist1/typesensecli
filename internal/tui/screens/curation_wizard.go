package screens

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/tui/components"
)

// curationWizard walks a user through building one curation item:
//
//	step 0: item id
//	step 1: pick rule type (query+match | filter_by | tags)
//	step 2: rule fields for the picked type
//	step 3: add includes (id + position) in a loop
//	step 4: add excludes (id) in a loop
//	step 5: preview JSON, Ctrl+S submits, Esc cancels
//
// Multi-item sets aren't supported — users who need that use the raw-JSON
// escape hatch (key `N` on the Curations tab).
type curationWizard struct {
	c    *client.Client
	name string
	step int

	width, height int

	itemID textinput.Model

	ruleIdx   int // 0=query+match, 1=filter_by, 2=tags
	query     textinput.Model
	matchIdx  int // 0=exact, 1=contains
	filterBy  textinput.Model
	tags      textinput.Model
	ruleFocus int // within step 2, which input is focused

	incID    textinput.Model
	incPos   textinput.Model
	incFocus int
	includes []curationInclude

	excID    textinput.Model
	excludes []string

	// Document picker modal. When non-nil all input is routed to it.
	// pickerTarget tracks which wizard field receives the chosen id:
	//   0 = incID, 1 = excID.
	picker       *components.DocPicker
	pickerTarget int

	preview []byte
	err     string

	submitted bool
	cancelled bool
}

type curationInclude struct {
	id       string
	position int
}

var ruleTypeLabels = []string{"query + match", "filter_by", "tags (comma-separated)"}
var matchLabels = []string{"exact", "contains"}

func newCurationWizard(c *client.Client, name string, w, h int) curationWizard {
	mk := func(ph string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		return t
	}
	wiz := curationWizard{
		c:        c,
		name:     name,
		width:    w,
		height:   h,
		itemID:   mk("item id (e.g. promote-brand-x)"),
		query:    mk("query text (e.g. brand x)"),
		filterBy: mk("filter expression (e.g. category:=shoes)"),
		tags:     mk("tag1, tag2"),
		incID:    mk("document id (Ctrl+F to search)"),
		incPos:   mk("position (1-based)"),
		excID:    mk("document id (Ctrl+F to search)"),
	}
	wiz.itemID.Focus()
	return wiz
}

func (w curationWizard) Submitted() []byte {
	if w.submitted {
		return w.preview
	}
	return nil
}

func (w curationWizard) Cancelled() bool { return w.cancelled }
func (w curationWizard) Name() string    { return w.name }

func (w curationWizard) Update(msg tea.Msg) (curationWizard, tea.Cmd) {
	// Document picker modal takes priority.
	if w.picker != nil {
		updated, cmd := w.picker.Update(msg)
		w.picker = &updated
		if id := w.picker.Picked(); id != "" {
			switch w.pickerTarget {
			case 0:
				w.incID.SetValue(id)
			case 1:
				w.excID.SetValue(id)
			}
			w.picker = nil
			return w, nil
		}
		if w.picker.Cancelled() {
			w.picker = nil
			return w, nil
		}
		return w, cmd
	}

	km, isKey := msg.(tea.KeyMsg)
	if isKey && km.String() == "esc" {
		w.cancelled = true
		return w, nil
	}

	switch w.step {
	case 0:
		return w.updateItemID(msg, km, isKey)
	case 1:
		return w.updateRulePicker(km, isKey)
	case 2:
		return w.updateRuleFields(msg, km, isKey)
	case 3:
		return w.updateIncludes(msg, km, isKey)
	case 4:
		return w.updateExcludes(msg, km, isKey)
	case 5:
		return w.updatePreview(km, isKey)
	}
	return w, nil
}

func (w curationWizard) updateItemID(msg tea.Msg, km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if isKey && km.String() == "enter" {
		if strings.TrimSpace(w.itemID.Value()) == "" {
			w.err = "item id is required"
			return w, nil
		}
		w.err = ""
		w.step = 1
		w.itemID.Blur()
		return w, nil
	}
	var cmd tea.Cmd
	w.itemID, cmd = w.itemID.Update(msg)
	return w, cmd
}

func (w curationWizard) updateRulePicker(km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if !isKey {
		return w, nil
	}
	switch km.String() {
	case "up", "k":
		if w.ruleIdx > 0 {
			w.ruleIdx--
		}
	case "down", "j":
		if w.ruleIdx < len(ruleTypeLabels)-1 {
			w.ruleIdx++
		}
	case "enter":
		w.step = 2
		w.ruleFocus = 0
		w.focusStep2()
	}
	return w, nil
}

func (w *curationWizard) focusStep2() {
	w.query.Blur()
	w.filterBy.Blur()
	w.tags.Blur()
	switch w.ruleIdx {
	case 0:
		w.query.Focus()
	case 1:
		w.filterBy.Focus()
	case 2:
		w.tags.Focus()
	}
}

func (w curationWizard) updateRuleFields(msg tea.Msg, km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if isKey {
		switch km.String() {
		case "enter":
			if !w.validateStep2() {
				return w, nil
			}
			w.err = ""
			w.step = 3
			w.query.Blur()
			w.filterBy.Blur()
			w.tags.Blur()
			w.incID.Focus()
			w.incFocus = 0
			return w, nil
		case "tab":
			if w.ruleIdx == 0 {
				// toggle match exact/contains via Tab on the query step
				w.matchIdx = 1 - w.matchIdx
				return w, nil
			}
		}
	}
	var cmd tea.Cmd
	switch w.ruleIdx {
	case 0:
		w.query, cmd = w.query.Update(msg)
	case 1:
		w.filterBy, cmd = w.filterBy.Update(msg)
	case 2:
		w.tags, cmd = w.tags.Update(msg)
	}
	return w, cmd
}

func (w *curationWizard) validateStep2() bool {
	switch w.ruleIdx {
	case 0:
		if strings.TrimSpace(w.query.Value()) == "" {
			w.err = "query is required"
			return false
		}
	case 1:
		if strings.TrimSpace(w.filterBy.Value()) == "" {
			w.err = "filter_by is required"
			return false
		}
	case 2:
		if strings.TrimSpace(w.tags.Value()) == "" {
			w.err = "at least one tag is required"
			return false
		}
	}
	return true
}

func (w curationWizard) updateIncludes(msg tea.Msg, km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if isKey {
		switch km.String() {
		case "ctrl+f":
			if w.incFocus == 0 {
				p := components.NewDocPicker(w.c, w.width, w.height, "", "")
				w.picker = &p
				w.pickerTarget = 0
				return w, textinput.Blink
			}
		case "tab", "shift+tab":
			w.incFocus = 1 - w.incFocus
			if w.incFocus == 0 {
				w.incID.Focus()
				w.incPos.Blur()
			} else {
				w.incID.Blur()
				w.incPos.Focus()
			}
			return w, nil
		case "enter":
			id := strings.TrimSpace(w.incID.Value())
			posStr := strings.TrimSpace(w.incPos.Value())
			if id == "" {
				// empty id = done adding includes
				w.step = 4
				w.incID.Blur()
				w.incPos.Blur()
				w.excID.Focus()
				w.err = ""
				return w, nil
			}
			pos, err := strconv.Atoi(posStr)
			if err != nil || pos < 1 {
				w.err = "position must be a positive integer"
				return w, nil
			}
			w.includes = append(w.includes, curationInclude{id: id, position: pos})
			w.incID.SetValue("")
			w.incPos.SetValue("")
			w.incID.Focus()
			w.incPos.Blur()
			w.incFocus = 0
			w.err = ""
			return w, nil
		}
	}
	var cmd tea.Cmd
	if w.incFocus == 0 {
		w.incID, cmd = w.incID.Update(msg)
	} else {
		w.incPos, cmd = w.incPos.Update(msg)
	}
	return w, cmd
}

func (w curationWizard) updateExcludes(msg tea.Msg, km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if isKey && km.String() == "ctrl+f" {
		p := components.NewDocPicker(w.c, w.width, w.height, "", "")
		w.picker = &p
		w.pickerTarget = 1
		return w, textinput.Blink
	}
	if isKey && km.String() == "enter" {
		id := strings.TrimSpace(w.excID.Value())
		if id == "" {
			// empty = done
			w.step = 5
			w.excID.Blur()
			w.preview = w.buildBody()
			return w, nil
		}
		w.excludes = append(w.excludes, id)
		w.excID.SetValue("")
		return w, nil
	}
	var cmd tea.Cmd
	w.excID, cmd = w.excID.Update(msg)
	return w, cmd
}

func (w curationWizard) updatePreview(km tea.KeyMsg, isKey bool) (curationWizard, tea.Cmd) {
	if !isKey {
		return w, nil
	}
	switch km.String() {
	case "ctrl+s":
		w.submitted = true
	case "b":
		// go back to excludes step to tweak
		w.step = 4
		w.excID.Focus()
	}
	return w, nil
}

func (w curationWizard) buildBody() []byte {
	var rule map[string]any
	switch w.ruleIdx {
	case 0:
		rule = map[string]any{
			"query": strings.TrimSpace(w.query.Value()),
			"match": matchLabels[w.matchIdx],
		}
	case 1:
		rule = map[string]any{"filter_by": strings.TrimSpace(w.filterBy.Value())}
	case 2:
		parts := strings.Split(w.tags.Value(), ",")
		tagList := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				tagList = append(tagList, t)
			}
		}
		rule = map[string]any{"tags": tagList}
	}

	includes := make([]map[string]any, 0, len(w.includes))
	for _, inc := range w.includes {
		includes = append(includes, map[string]any{"id": inc.id, "position": inc.position})
	}
	excludes := make([]map[string]any, 0, len(w.excludes))
	for _, id := range w.excludes {
		excludes = append(excludes, map[string]any{"id": id})
	}

	item := map[string]any{
		"id":       strings.TrimSpace(w.itemID.Value()),
		"rule":     rule,
		"includes": includes,
		"excludes": excludes,
	}
	body := map[string]any{"items": []any{item}}
	b, _ := json.MarshalIndent(body, "", "  ")
	return b
}

func (w curationWizard) View() string {
	if w.picker != nil {
		return w.picker.View()
	}
	header := fmt.Sprintf("Curation wizard — %q  (step %d/5)\n\n", w.name, w.step+1)

	var body string
	switch w.step {
	case 0:
		body = "Item id\n\n" + w.itemID.View() + "\n\nEnter continue · Esc cancel"
	case 1:
		lines := []string{"Pick rule type\n"}
		for i, lbl := range ruleTypeLabels {
			prefix := "  "
			if i == w.ruleIdx {
				prefix = "› "
			}
			lines = append(lines, prefix+lbl)
		}
		body = strings.Join(lines, "\n") + "\n\n↑/↓ choose · Enter continue · Esc cancel"
	case 2:
		switch w.ruleIdx {
		case 0:
			body = "Query rule\n\n" + w.query.View() +
				"\n\nMatch: " + matchLabels[w.matchIdx] + "  (Tab toggles)" +
				"\n\nEnter continue · Esc cancel"
		case 1:
			body = "Filter rule\n\n" + w.filterBy.View() + "\n\nEnter continue · Esc cancel"
		case 2:
			body = "Tag rule\n\n" + w.tags.View() + "\n\nEnter continue · Esc cancel"
		}
	case 3:
		body = "Includes (" + fmt.Sprint(len(w.includes)) + " added)\n\n" +
			"id:       " + w.incID.View() + "\n" +
			"position: " + w.incPos.View() + "\n\n" +
			"Enter adds row · blank id + Enter skips to next step · Tab switches field · Ctrl+F search · Esc cancel"
		for _, inc := range w.includes {
			body += fmt.Sprintf("\n  • %s @ %d", inc.id, inc.position)
		}
	case 4:
		body = "Excludes (" + fmt.Sprint(len(w.excludes)) + " added)\n\n" +
			"id: " + w.excID.View() + "\n\n" +
			"Enter adds row · blank id + Enter moves to preview · Ctrl+F search · Esc cancel"
		for _, id := range w.excludes {
			body += "\n  • " + id
		}
	case 5:
		body = "Preview\n\n" + string(w.preview) + "\n\nCtrl+S submit · b back · Esc cancel"
	}

	if w.err != "" {
		body += "\n\n⚠ " + w.err
	}
	return header + body
}
