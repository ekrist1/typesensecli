package screens

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// synonymWizard walks a user through building one synonym item:
//
//	step 0: item id
//	step 1: pick synonym type (multi-way | one-way)
//	step 2: synonym terms
//	step 3: optional locale and symbols_to_index
//	step 4: preview JSON, Ctrl+S submits, Esc cancels
//
// Multi-item sets are edited through the raw JSON path.
type synonymWizard struct {
	name string
	step int

	width, height int

	itemID textinput.Model

	typeIdx   int // 0=multi-way, 1=one-way
	root      textinput.Model
	synonyms  textinput.Model
	termFocus int

	locale   textinput.Model
	symbols  textinput.Model
	optFocus int

	preview []byte
	err     string

	submitted bool
	cancelled bool

	bodyExtra      map[string]any
	itemExtra      map[string]any
	initialTypeIdx int
}

var synonymTypeLabels = []string{"multi-way synonyms", "one-way synonyms"}

var errUnsupportedSynonymWizard = errors.New("wizard edit supports single-item synonym sets")

func newSynonymWizard(name string, w, h int) synonymWizard {
	mk := func(ph string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		return t
	}
	wiz := synonymWizard{
		name:     name,
		width:    w,
		height:   h,
		itemID:   mk("item id (e.g. coat-synonyms)"),
		root:     mk("root term (e.g. smart phone)"),
		synonyms: mk("comma-separated terms (e.g. blazer, coat, jacket)"),
		locale:   mk("locale (optional, e.g. en)"),
		symbols:  mk("symbols to index (optional, comma-separated, e.g. +, #)"),
	}
	wiz.itemID.Focus()
	return wiz
}

func newSynonymWizardFromBody(name string, body []byte, w, h int) (synonymWizard, error) {
	wiz := newSynonymWizard(name, w, h)

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return wiz, err
	}

	rawItems, ok := payload["items"].([]any)
	if !ok || len(rawItems) != 1 {
		return wiz, errUnsupportedSynonymWizard
	}

	item, ok := rawItems[0].(map[string]any)
	if !ok {
		return wiz, errUnsupportedSynonymWizard
	}

	itemID, ok := item["id"].(string)
	if !ok || strings.TrimSpace(itemID) == "" {
		return wiz, errUnsupportedSynonymWizard
	}
	wiz.itemID.SetValue(itemID)
	wiz.itemID.CursorEnd()

	synonyms := stringSliceValue(item["synonyms"])
	if len(synonyms) == 0 {
		return wiz, errUnsupportedSynonymWizard
	}
	wiz.synonyms.SetValue(strings.Join(synonyms, ", "))
	wiz.synonyms.CursorEnd()

	if root := stringValue(item, "root"); strings.TrimSpace(root) != "" {
		wiz.typeIdx = 1
		wiz.root.SetValue(root)
		wiz.root.CursorEnd()
	}
	wiz.initialTypeIdx = wiz.typeIdx

	if locale := stringValue(item, "locale"); strings.TrimSpace(locale) != "" {
		wiz.locale.SetValue(locale)
		wiz.locale.CursorEnd()
	}
	if symbols := stringListishValue(item["symbols_to_index"]); len(symbols) > 0 {
		wiz.symbols.SetValue(strings.Join(symbols, ", "))
		wiz.symbols.CursorEnd()
	}

	wiz.bodyExtra = copyMapWithoutKeys(payload, "items")
	wiz.itemExtra = copyMapWithoutKeys(item, "id", "root", "synonyms", "locale", "symbols_to_index")

	return wiz, nil
}

func (w synonymWizard) Submitted() []byte {
	if w.submitted {
		return w.preview
	}
	return nil
}

func (w synonymWizard) Cancelled() bool { return w.cancelled }
func (w synonymWizard) Name() string    { return w.name }

func (w synonymWizard) Update(msg tea.Msg) (synonymWizard, tea.Cmd) {
	km, isKey := msg.(tea.KeyMsg)
	if isKey && km.String() == "esc" {
		w.cancelled = true
		return w, nil
	}

	switch w.step {
	case 0:
		return w.updateSynonymItemID(msg, km, isKey)
	case 1:
		return w.updateSynonymTypePicker(km, isKey)
	case 2:
		return w.updateSynonymTerms(msg, km, isKey)
	case 3:
		return w.updateSynonymOptions(msg, km, isKey)
	case 4:
		return w.updateSynonymPreview(km, isKey)
	}
	return w, nil
}

func (w synonymWizard) updateSynonymItemID(msg tea.Msg, km tea.KeyMsg, isKey bool) (synonymWizard, tea.Cmd) {
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

func (w synonymWizard) updateSynonymTypePicker(km tea.KeyMsg, isKey bool) (synonymWizard, tea.Cmd) {
	if !isKey {
		return w, nil
	}
	switch km.String() {
	case "up", "k":
		if w.typeIdx > 0 {
			w.typeIdx--
		}
	case "down", "j":
		if w.typeIdx < len(synonymTypeLabels)-1 {
			w.typeIdx++
		}
	case "enter":
		w.step = 2
		w.termFocus = 0
		w.focusSynonymTerms()
	}
	return w, nil
}

func (w *synonymWizard) focusSynonymTerms() {
	w.root.Blur()
	w.synonyms.Blur()
	if w.typeIdx == 1 && w.termFocus == 0 {
		w.root.Focus()
		return
	}
	w.synonyms.Focus()
}

func (w synonymWizard) updateSynonymTerms(msg tea.Msg, km tea.KeyMsg, isKey bool) (synonymWizard, tea.Cmd) {
	if isKey {
		switch km.String() {
		case "tab", "shift+tab":
			if w.typeIdx == 1 {
				w.termFocus = 1 - w.termFocus
				w.focusSynonymTerms()
				return w, nil
			}
		case "enter":
			if !w.validateSynonymTerms() {
				return w, nil
			}
			w.err = ""
			w.step = 3
			w.root.Blur()
			w.synonyms.Blur()
			w.locale.Focus()
			w.symbols.Blur()
			w.optFocus = 0
			return w, nil
		}
	}
	var cmd tea.Cmd
	if w.typeIdx == 1 && w.termFocus == 0 {
		w.root, cmd = w.root.Update(msg)
	} else {
		w.synonyms, cmd = w.synonyms.Update(msg)
	}
	return w, cmd
}

func (w *synonymWizard) validateSynonymTerms() bool {
	terms := parseCommaSeparated(w.synonyms.Value())
	if w.typeIdx == 0 && len(terms) < 2 {
		w.err = "multi-way synonyms require at least two terms"
		return false
	}
	if w.typeIdx == 1 {
		if strings.TrimSpace(w.root.Value()) == "" {
			w.err = "root term is required"
			return false
		}
		if len(terms) == 0 {
			w.err = "at least one mapped synonym is required"
			return false
		}
	}
	return true
}

func (w synonymWizard) updateSynonymOptions(msg tea.Msg, km tea.KeyMsg, isKey bool) (synonymWizard, tea.Cmd) {
	if isKey {
		switch km.String() {
		case "tab", "shift+tab":
			w.optFocus = 1 - w.optFocus
			if w.optFocus == 0 {
				w.locale.Focus()
				w.symbols.Blur()
			} else {
				w.locale.Blur()
				w.symbols.Focus()
			}
			return w, nil
		case "enter":
			w.step = 4
			w.locale.Blur()
			w.symbols.Blur()
			w.preview = w.buildBody()
			return w, nil
		}
	}
	var cmd tea.Cmd
	if w.optFocus == 0 {
		w.locale, cmd = w.locale.Update(msg)
	} else {
		w.symbols, cmd = w.symbols.Update(msg)
	}
	return w, cmd
}

func (w synonymWizard) updateSynonymPreview(km tea.KeyMsg, isKey bool) (synonymWizard, tea.Cmd) {
	if !isKey {
		return w, nil
	}
	switch km.String() {
	case "ctrl+s":
		w.submitted = true
	case "b":
		w.step = 3
		w.locale.Focus()
		w.optFocus = 0
	}
	return w, nil
}

func (w synonymWizard) buildBody() []byte {
	item := copyMapWithoutKeys(w.itemExtra)
	item["id"] = strings.TrimSpace(w.itemID.Value())
	item["synonyms"] = parseCommaSeparated(w.synonyms.Value())
	if w.typeIdx == 1 {
		item["root"] = strings.TrimSpace(w.root.Value())
	}
	if locale := strings.TrimSpace(w.locale.Value()); locale != "" {
		item["locale"] = locale
	}
	if symbols := parseCommaSeparated(w.symbols.Value()); len(symbols) > 0 {
		item["symbols_to_index"] = symbols
	}

	body := copyMapWithoutKeys(w.bodyExtra)
	body["items"] = []any{item}
	b, _ := json.MarshalIndent(body, "", "  ")
	return b
}

func (w synonymWizard) View() string {
	header := fmt.Sprintf("Synonym wizard - %q  (step %d/5)\n\n", w.name, w.step+1)

	var body string
	switch w.step {
	case 0:
		body = "Item id\n\n" + w.itemID.View() + "\n\nEnter continue - Esc cancel"
	case 1:
		lines := []string{"Pick synonym type\n"}
		for i, lbl := range synonymTypeLabels {
			prefix := "  "
			if i == w.typeIdx {
				prefix = "> "
			}
			lines = append(lines, prefix+lbl)
		}
		body = strings.Join(lines, "\n") + "\n\nUp/Down choose - Enter continue - Esc cancel"
	case 2:
		if w.typeIdx == 1 {
			body = "One-way synonym\n\n" +
				"root:     " + w.root.View() + "\n" +
				"synonyms: " + w.synonyms.View() + "\n\n" +
				"Tab switches field - Enter continue - Esc cancel"
		} else {
			body = "Multi-way synonyms\n\n" + w.synonyms.View() + "\n\nEnter continue - Esc cancel"
		}
	case 3:
		body = "Options\n\n" +
			"locale:           " + w.locale.View() + "\n" +
			"symbols_to_index: " + w.symbols.View() + "\n\n" +
			"Tab switches field - Enter preview - Esc cancel"
	case 4:
		body = "Preview\n\n" + string(w.preview) + "\n\nCtrl+S submit - b back - Esc cancel"
	}

	if w.err != "" {
		body += "\n\n! " + w.err
	}
	return header + body
}

func parseCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func stringListishValue(raw any) []string {
	switch value := raw.(type) {
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if strings.TrimSpace(item) != "" {
				out = append(out, item)
			}
		}
		return out
	case string:
		return parseCommaSeparated(value)
	default:
		return nil
	}
}
