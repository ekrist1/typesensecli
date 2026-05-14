package screens

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
	"clisense/internal/tui/components"
)

const tagCollectionSearch = "collection:search"

type collectionSearchHit struct {
	id      string
	snippet string
	detail  []byte
}

func (h collectionSearchHit) Title() string       { return h.id }
func (h collectionSearchHit) Description() string { return h.snippet }
func (h collectionSearchHit) FilterValue() string { return h.id + " " + h.snippet }

type collectionSearchModal struct {
	c          *client.Client
	collection string

	queryBy       textinput.Model
	queryText     textinput.Model
	searchable    []string
	results       list.Model
	detail        components.JSONView
	focus         int
	status        string
	width, height int
	closed        bool
}

func newCollectionSearchModal(c *client.Client, collection string, searchable []string, w, h int) collectionSearchModal {
	leftWidth, rightWidth, resultsHeight, detailHeight := collectionSearchSizes(w, h)

	queryBy := textinput.New()
	queryBy.Placeholder = "fields to search (e.g. title,summary)"
	queryBy.Width = max(20, leftWidth-2)
	queryBy.CharLimit = 0
	if len(searchable) > 0 {
		queryBy.SetValue(defaultCollectionSearchField(searchable))
		queryBy.CursorEnd()
	}
	queryBy.Focus()

	queryText := textinput.New()
	queryText.Placeholder = "search text"
	queryText.Width = max(20, leftWidth-2)
	queryText.CharLimit = 0

	results := list.New(nil, list.NewDefaultDelegate(), leftWidth, resultsHeight)
	results.Title = "Results"
	results.SetShowStatusBar(false)
	results.SetShowHelp(false)
	results.SetFilteringEnabled(false)

	detail := components.NewJSONView(rightWidth, detailHeight)
	detail.SetContent([]byte("Run a search to inspect matching documents."))

	return collectionSearchModal{
		c:          c,
		collection: collection,
		queryBy:    queryBy,
		queryText:  queryText,
		searchable: append([]string(nil), searchable...),
		results:    results,
		detail:     detail,
		width:      w,
		height:     h,
	}
}

func (m *collectionSearchModal) SetSize(w, h int) {
	m.width, m.height = w, h
	leftWidth, rightWidth, resultsHeight, detailHeight := collectionSearchSizes(w, h)
	m.queryBy.Width = max(20, leftWidth-2)
	m.queryText.Width = max(20, leftWidth-2)
	m.results.SetSize(leftWidth, resultsHeight)
	m.detail.SetSize(rightWidth, detailHeight)
}

func (m collectionSearchModal) Closed() bool { return m.closed }

func (m collectionSearchModal) Update(msg tea.Msg) (collectionSearchModal, tea.Cmd) {
	switch v := msg.(type) {
	case api.SuccessMsg:
		if v.Tag == collectionSearchTag(m.collection) {
			hits, err := parseCollectionSearchHits(v.Body)
			if err != nil {
				m.status = "parse error: " + err.Error()
				return m, nil
			}
			items := make([]list.Item, 0, len(hits))
			for _, hit := range hits {
				items = append(items, hit)
			}
			m.results.SetItems(items)
			m.status = fmt.Sprintf("%d hits", len(items))
			if len(items) == 0 {
				m.detail.SetContent([]byte("No documents found."))
				return m, nil
			}
			m.focus = 2
			m.queryBy.Blur()
			m.queryText.Blur()
			m.syncResultPreview()
			return m, nil
		}
	case api.ErrorMsg:
		if v.Tag == collectionSearchTag(m.collection) {
			if v.Err != nil {
				m.status = "network error: " + v.Err.Error()
			} else {
				m.status = fmt.Sprintf("HTTP %d: %s", v.Status, truncateCollectionSearchString(string(v.Body), 160))
			}
			return m, nil
		}
	case tea.KeyMsg:
		switch v.String() {
		case "esc":
			m.closed = true
			return m, nil
		case "tab":
			m.cycleFocus(+1)
			return m, nil
		case "shift+tab":
			m.cycleFocus(-1)
			return m, nil
		case "ctrl+b":
			m.detail.PageUp()
			return m, nil
		case "ctrl+f":
			m.detail.PageDown()
			return m, nil
		case "enter":
			if m.focus == 0 || m.focus == 1 {
				return m.runSearch()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focus {
	case 0:
		m.queryBy, cmd = m.queryBy.Update(msg)
	case 1:
		m.queryText, cmd = m.queryText.Update(msg)
	case 2:
		m.results, cmd = m.results.Update(msg)
		m.syncResultPreview()
	}
	return m, cmd
}

func (m *collectionSearchModal) cycleFocus(dir int) {
	m.focus = (m.focus + dir + 3) % 3
	m.queryBy.Blur()
	m.queryText.Blur()
	switch m.focus {
	case 0:
		m.queryBy.Focus()
	case 1:
		m.queryText.Focus()
	}
}

func (m collectionSearchModal) runSearch() (collectionSearchModal, tea.Cmd) {
	queryBy := strings.TrimSpace(m.queryBy.Value())
	queryText := strings.TrimSpace(m.queryText.Value())
	if queryBy == "" {
		m.status = "search fields are required"
		m.focus = 0
		m.queryText.Blur()
		m.queryBy.Focus()
		return m, nil
	}
	if queryText == "" {
		m.status = "search text is required"
		m.focus = 1
		m.queryBy.Blur()
		m.queryText.Focus()
		return m, nil
	}
	m.status = "searching…"
	method, path := client.SearchDocuments(m.collection, queryText, queryBy)
	return m, api.DoRequest(m.c, collectionSearchTag(m.collection), method, path, nil)
}

func (m *collectionSearchModal) syncResultPreview() {
	if hit, ok := m.results.SelectedItem().(collectionSearchHit); ok {
		m.detail.SetContent(hit.detail)
	}
}

func (m collectionSearchModal) View() string {
	header := fmt.Sprintf("Search collection %q", m.collection)
	if m.status != "" {
		header += " · " + m.status
	}

	queryHint := "Available fields: "
	if len(m.searchable) == 0 {
		queryHint += "none detected from schema; enter query_by manually"
	} else {
		queryHint += truncateCollectionSearchString(strings.Join(m.searchable, ", "), 100)
	}

	leftLines := []string{
		header,
		"",
		"Search fields: " + m.queryBy.View(),
		lipgloss.NewStyle().Faint(true).Render(queryHint),
		"",
		"Search text:   " + m.queryText.View(),
		"",
		m.results.View(),
	}

	left := strings.Join(leftLines, "\n")
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", m.detail.View())
	footer := "Tab switch · Enter search · Ctrl+B/Ctrl+F scroll JSON · Esc close"
	return body + "\n" + footer
}

func collectionSearchSizes(totalWidth, totalHeight int) (leftWidth, rightWidth, resultsHeight, detailHeight int) {
	leftWidth, rightWidth = splitPaneWidths(totalWidth)
	bodyHeight := max(10, totalHeight-4)
	resultsHeight = max(4, bodyHeight-8)
	detailHeight = bodyHeight
	return leftWidth, rightWidth, resultsHeight, detailHeight
}

func collectionSearchTag(collection string) string {
	return tagCollectionSearch + ":" + collection
}

func defaultCollectionSearchField(fields []string) string {
	present := make(map[string]bool, len(fields))
	for _, f := range fields {
		present[f] = true
	}
	for _, f := range collectionSearchPreviewFields {
		if present[f] {
			return f
		}
	}
	return fields[0]
}

func searchableCollectionFields(fields []collectionFieldItem) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.typ == "string" || field.typ == "string[]" {
			out = append(out, field.name)
		}
	}
	return out
}

func parseCollectionSearchHits(body []byte) ([]collectionSearchHit, error) {
	var resp struct {
		Hits []struct {
			Document map[string]any `json:"document"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	hits := make([]collectionSearchHit, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		id, _ := hit.Document["id"].(string)
		detail, err := json.MarshalIndent(hit.Document, "", "  ")
		if err != nil {
			return nil, err
		}
		hits = append(hits, collectionSearchHit{
			id:      id,
			snippet: collectionSearchPreview(hit.Document),
			detail:  detail,
		})
	}
	return hits, nil
}

var collectionSearchPreviewFields = []string{
	"title", "name", "heading", "headline", "subject", "label",
	"summary", "description", "body", "content", "text",
}

var collectionSearchNoiseFields = map[string]bool{
	"id": true, "collection_handle": true, "handle": true,
	"slug": true, "type": true, "status": true, "locale": true,
}

func collectionSearchPreview(doc map[string]any) string {
	for _, key := range collectionSearchPreviewFields {
		if value, ok := doc[key].(string); ok && strings.TrimSpace(value) != "" {
			return key + ": " + truncateCollectionSearchString(value, 80)
		}
	}

	keys := make([]string, 0, len(doc))
	for key := range doc {
		if collectionSearchNoiseFields[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value, ok := doc[key].(string); ok && strings.TrimSpace(value) != "" {
			return key + ": " + truncateCollectionSearchString(value, 80)
		}
	}
	return ""
}

func truncateCollectionSearchString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
