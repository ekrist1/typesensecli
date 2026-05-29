package screens

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clisense/internal/client"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
	"clisense/internal/tui/theme"
)

// panel describes a read-only cluster endpoint shown in the left list.
type panel struct {
	title string
	tag   string
	call  func() (string, string)
}

// op describes a confirm-gated POST operation.
type op struct {
	key   string // trigger key
	tag   string
	label string // confirm prompt
	call  func() (string, string)
}

type clusterPanelItem struct{ title string }

func (i clusterPanelItem) Title() string       { return i.title }
func (i clusterPanelItem) Description() string { return "" }
func (i clusterPanelItem) FilterValue() string { return i.title }

const snapshotPath = "/tmp/typesense-snapshot"

// Cluster is the cluster-operations dashboard: read-only panels on the left, a
// JSON detail pane on the right, plus confirm-gated POST operations.
type Cluster struct {
	c       *client.Client
	list    list.Model
	detail  components.JSONView
	confirm *components.Confirm

	panels   []panel
	ops      []op
	payloads map[string][]byte // tag -> last response body

	status                string
	width, height         int
	listW, detailW, paneH int
}

func NewCluster(c *client.Client, w, h int) Cluster {
	panels := []panel{
		{title: "Health", tag: "Cluster:health", call: client.Health},
		{title: "Stats", tag: "Cluster:stats", call: client.Stats},
		{title: "Metrics", tag: "Cluster:metrics", call: client.Metrics},
		{title: "Debug", tag: "Cluster:debug", call: client.Debug},
	}
	ops := []op{
		{key: "S", tag: "Cluster:op:snapshot", label: "Create a snapshot at " + snapshotPath + "?", call: func() (string, string) { return client.Snapshot(snapshotPath) }},
		{key: "C", tag: "Cluster:op:compact", label: "Compact the on-disk database?", call: client.CompactDB},
		{key: "V", tag: "Cluster:op:vote", label: "Trigger leader re-election (vote)?", call: client.Vote},
		{key: "X", tag: "Cluster:op:clear-cache", label: "Clear the search cache?", call: client.ClearCache},
	}

	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	items := make([]list.Item, 0, len(panels))
	for _, p := range panels {
		items = append(items, clusterPanelItem{title: p.title})
	}
	l := list.New(items, list.NewDefaultDelegate(), max(1, listWidth-2), max(1, paneHeight-2))
	l.Title = "Cluster"
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	detail := components.NewJSONView(max(1, detailWidth-2), max(1, paneHeight-2))
	detail.SetContent([]byte("Press r to load cluster info."))

	return Cluster{
		c:        c,
		list:     l,
		detail:   detail,
		panels:   panels,
		ops:      ops,
		payloads: map[string][]byte{},
		width:    w, height: h,
		listW: listWidth, detailW: detailWidth, paneH: paneHeight,
	}
}

func (s *Cluster) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	s.listW, s.detailW, s.paneH = listWidth, detailWidth, paneHeight
	s.list.SetSize(max(1, listWidth-2), max(1, paneHeight-2))
	s.detail.SetSize(max(1, detailWidth-2), max(1, paneHeight-2))
}

func (s Cluster) HasModal() bool { return s.confirm != nil }

// Init fetches all read-only panels once on activation.
func (s Cluster) Init() tea.Cmd { return s.refreshAll() }

func (s Cluster) refreshAll() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(s.panels))
	for _, p := range s.panels {
		m, path := p.call()
		cmds = append(cmds, api.DoRequest(s.c, p.tag, m, path, nil))
	}
	return tea.Batch(cmds...)
}

func (s Cluster) selectedPanelTag() string {
	if it, ok := s.list.SelectedItem().(clusterPanelItem); ok {
		for _, p := range s.panels {
			if p.title == it.title {
				return p.tag
			}
		}
	}
	return ""
}

func (s Cluster) selectedPanelTitle() string {
	if it, ok := s.list.SelectedItem().(clusterPanelItem); ok {
		return it.title
	}
	return "Detail"
}

func (s *Cluster) showSelectedPayload() {
	if body, ok := s.payloads[s.selectedPanelTag()]; ok && body != nil {
		s.detail.SetContent(body)
	}
}

func (s Cluster) Update(msg tea.Msg) (Cluster, tea.Cmd) {
	// Confirm modal owns input until resolved.
	if s.confirm != nil {
		cmd := s.confirm.Update(msg)
		if res, ok := msg.(components.ConfirmResultMsg); ok {
			for _, o := range s.ops {
				if res.Tag == o.tag {
					s.confirm = nil
					if res.Confirmed {
						m, path := o.call()
						s.status = "running " + o.label
						return s, api.DoRequest(s.c, o.tag, m, path, nil)
					}
					return s, nil
				}
			}
		}
		return s, cmd
	}

	switch m := msg.(type) {
	case api.SuccessMsg:
		// Panel read result.
		for _, p := range s.panels {
			if m.Tag == p.tag {
				s.payloads[p.tag] = m.Body
				if s.selectedPanelTag() == p.tag {
					s.detail.SetContent(m.Body)
				}
				return s, nil
			}
		}
		// Operation result — show response in the detail pane.
		for _, o := range s.ops {
			if m.Tag == o.tag {
				s.status = "OK: " + o.label
				s.detail.SetContent(m.Body)
				return s, nil
			}
		}
	case api.ErrorMsg:
		s.status = formatAPIError(m)
		return s, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.refreshAll()
		case "pgup":
			s.detail.PageUp()
			return s, nil
		case "pgdown":
			s.detail.PageDown()
			return s, nil
		}
		// Operation trigger keys open a confirm dialog.
		for _, o := range s.ops {
			if m.String() == o.key {
				cf := components.NewConfirm(o.label, o.tag)
				s.confirm = &cf
				return s, nil
			}
		}
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	s.showSelectedPayload()
	return s, cmd
}

func (s Cluster) View() string {
	if s.confirm != nil {
		return s.confirm.View()
	}
	footer := theme.KeyHints(
		[2]string{"r", "refresh"}, [2]string{"S", "snapshot"}, [2]string{"C", "compact"},
		[2]string{"V", "vote"}, [2]string{"X", "clear-cache"}, [2]string{"PgUp/PgDn", "scroll"}, [2]string{"q", "quit"},
	)
	if s.status != "" {
		footer = theme.Status(statusKind(s.status), s.status) + theme.StatusBar.Render(" · ") + footer
	}
	listPanel := theme.Panel{Title: "Cluster", Width: s.listW, Height: s.paneH, Focused: true}
	detailPanel := theme.Panel{Title: s.selectedPanelTitle(), Width: s.detailW, Height: s.paneH}
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel.Render(s.list.View()), " ", detailPanel.Render(s.detail.View()))
	return body + "\n" + footer
}
