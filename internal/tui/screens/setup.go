// Package screens contains the tab-level Bubble Tea models.
package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui/theme"
)

type SetupDoneMsg struct {
	Cfg config.Config
}

type Setup struct {
	cfg       config.Config
	cluster   textinput.Model
	url       textinput.Model
	apiKey    textinput.Model
	focus     int // 0 = cluster, 1 = url, 2 = apiKey
	err       string
	clusters  []string
	clusterIx int // len(clusters) means the "new cluster" slot
}

func NewSetup(initial config.Config) Setup {
	names := initial.ClusterNames()
	active := initial.ActiveClusterName()
	if active == "" {
		active = "default"
	}
	cluster := textinput.New()
	cluster.Placeholder = "default"
	cluster.SetValue(active)

	url := textinput.New()
	url.Placeholder = "http://localhost:8108"
	url.SetValue(initial.URL())

	key := textinput.New()
	key.Placeholder = "API key"
	key.SetValue(initial.APIKey())
	key.EchoMode = textinput.EchoPassword

	clusterIx := 0
	for i, name := range names {
		if name == active {
			clusterIx = i
			break
		}
	}

	focus := 1
	if len(names) > 1 {
		focus = 0
	}
	s := Setup{cfg: initial, cluster: cluster, url: url, apiKey: key, focus: focus, clusters: names, clusterIx: clusterIx}
	s.focusInput()
	return s
}

func (s Setup) Init() tea.Cmd { return textinput.Blink }

func (s *Setup) SetError(msg string) { s.err = msg }

func (s Setup) Update(msg tea.Msg) (Setup, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "down":
			s.focus = (s.focus + 1) % 3
			s.focusInput()
			return s, nil
		case "shift+tab", "up":
			s.focus = (s.focus + 2) % 3
			s.focusInput()
			return s, nil
		case "left":
			if s.focus == 0 && len(s.clusters) > 0 {
				s.cycleCluster(-1)
				return s, nil
			}
		case "right":
			if s.focus == 0 && len(s.clusters) > 0 {
				s.cycleCluster(1)
				return s, nil
			}
		case "enter":
			if s.cluster.Value() == "" || s.url.Value() == "" || s.apiKey.Value() == "" {
				s.err = "cluster, URL, and API key are required"
				return s, nil
			}
			cfg := s.cfg
			cfg.SetConnection(s.cluster.Value(), s.url.Value(), s.apiKey.Value())
			return s, func() tea.Msg { return SetupDoneMsg{Cfg: cfg} }
		}
	}
	var cmd tea.Cmd
	switch s.focus {
	case 0:
		s.cluster, cmd = s.cluster.Update(msg)
	case 1:
		s.url, cmd = s.url.Update(msg)
	case 2:
		s.apiKey, cmd = s.apiKey.Update(msg)
	}
	return s, cmd
}

func (s *Setup) focusInput() {
	s.cluster.Blur()
	s.url.Blur()
	s.apiKey.Blur()
	switch s.focus {
	case 0:
		s.cluster.Focus()
	case 1:
		s.url.Focus()
	case 2:
		s.apiKey.Focus()
	}
}

func (s *Setup) cycleCluster(delta int) {
	total := len(s.clusters) + 1
	s.clusterIx = (s.clusterIx + delta + total) % total
	if s.clusterIx == len(s.clusters) {
		s.cluster.Placeholder = "new cluster name"
		s.cluster.SetValue("")
		s.url.SetValue("")
		s.apiKey.SetValue("")
		return
	}
	s.cluster.Placeholder = "default"
	name := s.clusters[s.clusterIx]
	s.cluster.SetValue(name)
	if cluster, ok := s.cfg.Clusters[name]; ok {
		s.url.SetValue(cluster.URL)
		s.apiKey.SetValue(cluster.APIKey)
	}
}

func (s Setup) View() string {
	v := theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — Typesense connection") + "\n\n"
	clusterLine := s.cluster.View()
	if s.clusterIx == len(s.clusters) && len(s.clusters) > 0 {
		clusterLine += theme.HeaderMeta.Render("  + new")
	}
	v += theme.HeaderMeta.Render("Cluster: ") + clusterLine + "\n"
	v += theme.HeaderMeta.Render("URL:     ") + s.url.View() + "\n"
	v += theme.HeaderMeta.Render("API key: ") + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += theme.StatusErr.Render(s.err) + "\n"
	}
	hints := [][2]string{{"Tab/↑↓", "field"}}
	if len(s.clusters) > 0 {
		hints = append(hints, [2]string{"←→", "cluster"})
	}
	hints = append(hints, [2]string{"Enter", "save"}, [2]string{"Ctrl+C", "quit"})
	v += theme.KeyHints(hints...)
	return v
}
