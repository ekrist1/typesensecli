package components

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clisense/internal/client"
	"clisense/internal/tui/api"
)

// DocPicker is a modal component that lets the user search a Typesense
// collection and pick a single document id. The host forwards all tea.Msgs
// to it and polls Picked() / Cancelled() after each Update to know when to
// close it.
//
// Flow: pick a collection (auto-fetched on open) → enter search fields →
// enter search text → pick from results.
type DocPicker struct {
	c *client.Client

	collections list.Model // focus 0
	searchIn    textinput.Model
	searchText  textinput.Model
	focus       int // 0=collections, 1=searchIn, 2=searchText, 3=results

	selectedCollection string
	schemaFields       []string // string/string[] field names from collection schema

	results list.Model
	status  string

	width, height int

	pickedID  string
	cancelled bool
}

// Session-scoped cache so the picker remembers the last search configuration.
var (
	lastCollection string
	lastSearchIn   string
)

const (
	tagDocPickerList   = "docpicker:list"
	tagDocPickerSchema = "docpicker:schema"
	tagDocPickerSearch = "docpicker:search"
)

type pickerCollection struct{ name string }

func (p pickerCollection) Title() string       { return p.name }
func (p pickerCollection) Description() string { return "" }
func (p pickerCollection) FilterValue() string { return p.name }

type docHit struct {
	id      string
	snippet string
}

func (d docHit) Title() string       { return d.id }
func (d docHit) Description() string { return d.snippet }
func (d docHit) FilterValue() string { return d.id }

// NewDocPicker constructs the picker and a command that fetches the list of
// collections. The caller should return both from its Update method so the
// picker is populated on first render.
func NewDocPicker(c *client.Client, w, h int) (DocPicker, tea.Cmd) {
	inputWidth := w - 20
	if inputWidth < 20 {
		inputWidth = 20
	}
	mk := func(ph, val string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		t.Width = inputWidth
		t.CharLimit = 0
		if val != "" {
			t.SetValue(val)
			t.CursorEnd()
		}
		return t
	}

	listH := h - 14
	if listH < 4 {
		listH = 4
	}

	colList := list.New(nil, list.NewDefaultDelegate(), w-4, listH)
	colList.Title = "Collections"
	colList.SetShowStatusBar(false)
	colList.SetFilteringEnabled(true)

	resList := list.New(nil, list.NewDefaultDelegate(), w-4, listH)
	resList.Title = "Results"
	resList.SetShowStatusBar(false)
	resList.SetFilteringEnabled(false)

	p := DocPicker{
		c:           c,
		collections: colList,
		searchIn:    mk("fields to search (e.g. name, description)", lastSearchIn),
		searchText:  mk("search text", ""),
		results:     resList,
		width:       w,
		height:      h,
		focus:       0,
	}

	m, pth := client.ListCollections()
	return p, api.DoRequest(c, tagDocPickerList, m, pth, nil)
}

func (d *DocPicker) SetSize(w, h int) {
	d.width, d.height = w, h
	listH := h - 14
	if listH < 4 {
		listH = 4
	}
	d.collections.SetSize(w-4, listH)
	d.results.SetSize(w-4, listH)
}

func (d DocPicker) Picked() string  { return d.pickedID }
func (d DocPicker) Cancelled() bool { return d.cancelled }

func (d DocPicker) Update(msg tea.Msg) (DocPicker, tea.Cmd) {
	switch m := msg.(type) {
	case api.SuccessMsg:
		switch m.Tag {
		case tagDocPickerList:
			names := parseCollectionNames(m.Body)
			items := make([]list.Item, 0, len(names))
			for _, n := range names {
				items = append(items, pickerCollection{name: n})
			}
			d.collections.SetItems(items)
			// If we have a cached collection, preselect it.
			for i, it := range items {
				if c, ok := it.(pickerCollection); ok && c.name == lastCollection {
					d.collections.Select(i)
					break
				}
			}
			d.status = fmt.Sprintf("%d collections", len(items))
			return d, nil
		case tagDocPickerSchema:
			d.schemaFields = parseSchemaStringFields(m.Body)
			// Pre-fill with ONE sensible default: the first preferred field
			// that exists, else the first schema field. Dumping every string
			// field (including nested `a.b.c` entries) makes the input
			// unreadable and often references fields that aren't indexed on
			// their own. The user can broaden from the hint below.
			if strings.TrimSpace(d.searchIn.Value()) == "" && len(d.schemaFields) > 0 {
				d.searchIn.SetValue(defaultSearchField(d.schemaFields))
				d.searchIn.CursorEnd()
			}
			return d, nil
		case tagDocPickerSearch:
			hits := parseHits(m.Body)
			items := make([]list.Item, 0, len(hits))
			for _, h := range hits {
				items = append(items, h)
			}
			d.results.SetItems(items)
			d.status = fmt.Sprintf("%d hits", len(hits))
			if len(items) > 0 {
				d.blurInputs()
				d.focus = 3
			}
			return d, nil
		}
	case api.ErrorMsg:
		if m.Tag == tagDocPickerList || m.Tag == tagDocPickerSchema || m.Tag == tagDocPickerSearch {
			if m.Err != nil {
				d.status = "network error: " + m.Err.Error()
			} else {
				d.status = fmt.Sprintf("HTTP %d: %s", m.Status, truncate(string(m.Body), 160))
			}
			return d, nil
		}
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			d.cancelled = true
			return d, nil
		case "tab":
			d.cycleFocus(+1)
			return d, nil
		case "shift+tab":
			d.cycleFocus(-1)
			return d, nil
		case "enter":
			return d.onEnter()
		}
	}

	var cmd tea.Cmd
	switch d.focus {
	case 0:
		d.collections, cmd = d.collections.Update(msg)
	case 1:
		d.searchIn, cmd = d.searchIn.Update(msg)
	case 2:
		d.searchText, cmd = d.searchText.Update(msg)
	case 3:
		d.results, cmd = d.results.Update(msg)
	}
	return d, cmd
}

func (d DocPicker) onEnter() (DocPicker, tea.Cmd) {
	switch d.focus {
	case 0:
		if it, ok := d.collections.SelectedItem().(pickerCollection); ok {
			d.selectedCollection = it.name
			lastCollection = it.name
			d.focus = 1
			d.searchIn.Focus()
			d.status = "collection: " + it.name
			m, p := client.GetCollection(it.name)
			return d, api.DoRequest(d.c, tagDocPickerSchema, m, p, nil)
		}
		return d, nil
	case 1, 2:
		coll := d.selectedCollection
		qb := strings.TrimSpace(d.searchIn.Value())
		q := strings.TrimSpace(d.searchText.Value())
		if coll == "" {
			d.status = "pick a collection first"
			d.blurInputs()
			d.focus = 0
			return d, nil
		}
		if qb == "" {
			d.status = "search fields are required"
			d.blurInputs()
			d.focus = 1
			d.searchIn.Focus()
			return d, nil
		}
		if q == "" {
			d.status = "search text is required"
			d.blurInputs()
			d.focus = 2
			d.searchText.Focus()
			return d, nil
		}
		lastSearchIn = qb
		d.status = "searching…"
		m, p := client.SearchDocuments(coll, q, qb)
		return d, api.DoRequest(d.c, tagDocPickerSearch, m, p, nil)
	case 3:
		if it, ok := d.results.SelectedItem().(docHit); ok {
			d.pickedID = it.id
		}
		return d, nil
	}
	return d, nil
}

func (d *DocPicker) blurInputs() {
	d.searchIn.Blur()
	d.searchText.Blur()
}

func (d *DocPicker) cycleFocus(dir int) {
	max := 2
	if len(d.results.Items()) > 0 {
		max = 3
	}
	d.focus = (d.focus + dir + (max + 1)) % (max + 1)
	d.blurInputs()
	switch d.focus {
	case 1:
		d.searchIn.Focus()
	case 2:
		d.searchText.Focus()
	}
}

var dimStyle = lipgloss.NewStyle().Faint(true)

func (d DocPicker) View() string {
	header := "Document picker"
	if d.status != "" {
		header += "  —  " + d.status
	}

	lines := []string{header, ""}

	switch d.focus {
	case 0:
		lines = append(lines,
			"Pick a collection:",
			"",
			d.collections.View(),
			"",
			dimStyle.Render("↑/↓ choose · Enter select · Tab skip · Esc cancel"),
		)
	case 1, 2:
		hint := "   (which fields to search — comma-separated)"
		if len(d.schemaFields) > 0 {
			hint = "   available: " + strings.Join(d.schemaFields, ", ")
		}
		lines = append(lines,
			"Collection: "+d.selectedCollection,
			"",
			"Search fields: "+d.searchIn.View(),
			dimStyle.Render(hint),
			"",
			"Search text:  "+d.searchText.View(),
			"",
			dimStyle.Render("Tab cycle · Enter run search · Esc cancel"),
		)
	case 3:
		lines = append(lines,
			"Collection: "+d.selectedCollection+"   Fields: "+d.searchIn.Value()+"   Text: "+d.searchText.Value(),
			"",
			d.results.View(),
			"",
			dimStyle.Render("↑/↓ choose · Enter pick · Tab back to search · Esc cancel"),
		)
	}

	return strings.Join(lines, "\n")
}

func parseCollectionNames(body []byte) []string {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil
	}
	names := make([]string, 0, len(arr))
	for _, c := range arr {
		if n, ok := c["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

func parseHits(body []byte) []docHit {
	var resp struct {
		Hits []struct {
			Document map[string]any `json:"document"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	hits := make([]docHit, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		id, _ := h.Document["id"].(string)
		hits = append(hits, docHit{id: id, snippet: firstStringField(h.Document)})
	}
	return hits
}

// preferredPreviewFields are tried first when building a list snippet. These
// are the conventional "headline" fields most collections carry.
var preferredPreviewFields = []string{
	"title", "name", "heading", "headline", "subject", "label",
	"summary", "description", "body", "content", "text",
}

// noisePreviewFields are skipped in the fallback path — they're usually
// category/handle values, not the actual content the user is searching for.
var noisePreviewFields = map[string]bool{
	"id": true, "collection_handle": true, "handle": true,
	"slug": true, "type": true, "status": true, "locale": true,
}

// firstStringField returns a "field: value" preview for list display. It
// prefers conventional title/content fields, falling back to the first
// alphabetical non-noise string field so entries from arbitrary collections
// still show something recognizable.
func firstStringField(doc map[string]any) string {
	for _, k := range preferredPreviewFields {
		if s, ok := doc[k].(string); ok && strings.TrimSpace(s) != "" {
			return k + ": " + truncate(s, 80)
		}
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		if noisePreviewFields[k] {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s, ok := doc[k].(string); ok && strings.TrimSpace(s) != "" {
			return k + ": " + truncate(s, 80)
		}
	}
	return ""
}

// defaultSearchField picks one field name as the initial query_by. It prefers
// the conventional headline fields (title, name, heading…) and falls back to
// the first field in the schema. Callers should only invoke this when
// schemaFields is non-empty.
func defaultSearchField(schemaFields []string) string {
	present := make(map[string]bool, len(schemaFields))
	for _, f := range schemaFields {
		present[f] = true
	}
	for _, f := range preferredPreviewFields {
		if present[f] {
			return f
		}
	}
	return schemaFields[0]
}

// parseSchemaStringFields pulls the names of fields whose type is "string" or
// "string[]" — the ones Typesense considers text-searchable by default.
func parseSchemaStringFields(body []byte) []string {
	var resp struct {
		Fields []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	out := make([]string, 0, len(resp.Fields))
	for _, f := range resp.Fields {
		if f.Type == "string" || f.Type == "string[]" {
			out = append(out, f.Name)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
