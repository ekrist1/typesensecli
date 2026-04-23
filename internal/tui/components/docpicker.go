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
// collection and pick a single document id. It is self-contained: the host
// forwards all tea.Msgs to it, and polls Picked() / Cancelled() after each
// Update to know when to close.
type DocPicker struct {
	c *client.Client

	collection textinput.Model
	queryBy    textinput.Model
	query      textinput.Model
	focus      int // 0=collection, 1=queryBy, 2=query, 3=results

	results list.Model
	status  string

	width, height int

	pickedID  string
	cancelled bool
}

// Session-scoped cache so the picker remembers collection + query_by the user
// last searched. Reset on process exit.
var (
	lastCollection string
	lastQueryBy    string
)

const tagDocPicker = "docpicker:search"

type docHit struct {
	id      string
	snippet string
}

func (d docHit) Title() string       { return d.id }
func (d docHit) Description() string { return d.snippet }
func (d docHit) FilterValue() string { return d.id }

// NewDocPicker builds a picker. defaultCollection / defaultQueryBy are used
// when the session cache is empty — typically the caller passes "" for both
// and the picker falls back to prior in-session values.
func NewDocPicker(c *client.Client, w, h int, defaultCollection, defaultQueryBy string) DocPicker {
	mk := func(ph, val string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		if val != "" {
			t.SetValue(val)
		}
		return t
	}
	coll := lastCollection
	if coll == "" {
		coll = defaultCollection
	}
	qb := lastQueryBy
	if qb == "" {
		qb = defaultQueryBy
	}

	p := DocPicker{
		c:          c,
		collection: mk("collection name", coll),
		queryBy:    mk("query_by (comma-separated fields)", qb),
		query:      mk("search text", ""),
		width:      w,
		height:     h,
	}
	p.results = list.New(nil, list.NewDefaultDelegate(), w-4, h-12)
	p.results.Title = "Results"
	p.results.SetShowStatusBar(false)
	p.results.SetFilteringEnabled(false)

	// Focus query if we have prefilled collection + query_by — that's the
	// common case on a second open.
	if coll != "" && qb != "" {
		p.focus = 2
		p.query.Focus()
	} else {
		p.collection.Focus()
	}
	return p
}

func (d *DocPicker) SetSize(w, h int) {
	d.width, d.height = w, h
	d.results.SetSize(w-4, h-12)
}

func (d DocPicker) Picked() string  { return d.pickedID }
func (d DocPicker) Cancelled() bool { return d.cancelled }

func (d DocPicker) Update(msg tea.Msg) (DocPicker, tea.Cmd) {
	switch m := msg.(type) {
	case api.SuccessMsg:
		if m.Tag == tagDocPicker {
			hits := parseHits(m.Body)
			items := make([]list.Item, 0, len(hits))
			for _, h := range hits {
				items = append(items, h)
			}
			d.results.SetItems(items)
			d.status = fmt.Sprintf("%d hits", len(hits))
			if len(items) > 0 {
				d.blurAll()
				d.focus = 3
			}
			return d, nil
		}
	case api.ErrorMsg:
		if m.Tag == tagDocPicker {
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
			if d.focus == 3 {
				if it, ok := d.results.SelectedItem().(docHit); ok {
					d.pickedID = it.id
					return d, nil
				}
				return d, nil
			}
			// Any non-results focus + Enter = fire the search.
			coll := strings.TrimSpace(d.collection.Value())
			qb := strings.TrimSpace(d.queryBy.Value())
			q := strings.TrimSpace(d.query.Value())
			if coll == "" || qb == "" || q == "" {
				d.status = "collection, query_by, and query are all required"
				return d, nil
			}
			lastCollection, lastQueryBy = coll, qb
			d.status = "searching…"
			m, p := client.SearchDocuments(coll, q, qb)
			return d, api.DoRequest(d.c, tagDocPicker, m, p, nil)
		}
	}

	var cmd tea.Cmd
	switch d.focus {
	case 0:
		d.collection, cmd = d.collection.Update(msg)
	case 1:
		d.queryBy, cmd = d.queryBy.Update(msg)
	case 2:
		d.query, cmd = d.query.Update(msg)
	case 3:
		d.results, cmd = d.results.Update(msg)
	}
	return d, cmd
}

func (d *DocPicker) blurAll() {
	d.collection.Blur()
	d.queryBy.Blur()
	d.query.Blur()
}

func (d *DocPicker) cycleFocus(dir int) {
	max := 3
	if len(d.results.Items()) == 0 {
		max = 2
	}
	d.focus = (d.focus + dir + (max + 1)) % (max + 1)
	d.blurAll()
	switch d.focus {
	case 0:
		d.collection.Focus()
	case 1:
		d.queryBy.Focus()
	case 2:
		d.query.Focus()
	}
}

var dimStyle = lipgloss.NewStyle().Faint(true)

func (d DocPicker) View() string {
	header := "Document picker"
	if d.status != "" {
		header += "  —  " + d.status
	}
	lines := []string{
		header,
		"",
		"Collection: " + d.collection.View(),
		"Query by:   " + d.queryBy.View(),
		"Query:      " + d.query.View(),
		"",
	}
	if len(d.results.Items()) > 0 {
		lines = append(lines, d.results.View())
	} else {
		lines = append(lines, dimStyle.Render("(no results yet — fill fields and press Enter)"))
	}
	lines = append(lines, "", "Tab cycle · Enter search/pick · Esc cancel")
	return strings.Join(lines, "\n")
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

// firstStringField returns a deterministic "field: value" preview for list
// display — picks the alphabetically-first string field other than id.
func firstStringField(doc map[string]any) string {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		if k == "id" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s, ok := doc[k].(string); ok {
			return k + ": " + truncate(s, 80)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
