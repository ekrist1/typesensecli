package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/config"
	"clisense/internal/tui/components"
	"clisense/internal/tui/screens"
)

type App struct {
	cfg     config.Config
	cfgPath string
	c       *client.Client
	inSetup bool
	setup   screens.Setup
	active  int
	width   int
	height  int

	collections   screens.Collections
	nlModels      screens.Resource
	curations     screens.Curations
	conversations screens.Conversations
	settings      screens.Settings

	tabs []string
}

func NewApp(cfg config.Config, cfgPath string, inSetup bool) App {
	a := App{
		cfg:     cfg,
		cfgPath: cfgPath,
		inSetup: inSetup,
		setup:   screens.NewSetup(cfg),
		tabs:    []string{"Collections", "NL Models", "Curations", "Conversations", "Settings"},
		width:   100,
		height:  30,
	}
	if !inSetup {
		a.c = client.New(cfg.URL, cfg.APIKey)
		a.buildTabs()
	}
	return a
}

func (a *App) buildTabs() {
	a.collections = screens.NewCollections(a.c, a.width, a.height-3)
	a.nlModels = screens.NewNLModels(a.c, a.width, a.height-3)
	a.curations = screens.NewCurations(a.c, a.width, a.height-3)
	a.conversations = screens.NewConversations(a.c, a.width, a.height-3)
	a.settings = screens.NewSettings(a.cfg)
}

func (a App) Init() tea.Cmd {
	if a.inSetup {
		return a.setup.Init()
	}
	return a.collections.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		if !a.inSetup {
			a.collections.SetSize(a.width, a.height-3)
			a.nlModels.SetSize(a.width, a.height-3)
			a.curations.SetSize(a.width, a.height-3)
			a.conversations.SetSize(a.width, a.height-3)
		}
		return a, nil
	case screens.SetupDoneMsg:
		a.cfg = m.Cfg
		if err := config.Save(a.cfgPath, a.cfg); err != nil {
			return a, nil
		}
		a.c = client.New(a.cfg.URL, a.cfg.APIKey)
		a.inSetup = false
		a.buildTabs()
		return a, a.collections.Init()
	case tea.KeyMsg:
		if a.inSetup {
			var cmd tea.Cmd
			a.setup, cmd = a.setup.Update(msg)
			return a, cmd
		}
		switch m.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "1":
			a.active = 0
			return a, a.collections.Init()
		case "2":
			a.active = 1
			return a, a.nlModels.Init()
		case "3":
			a.active = 2
			return a, a.curations.Init()
		case "4":
			a.active = 3
			return a, a.conversations.Init()
		case "5":
			a.active = 4
			return a, a.settings.Init()
		case "tab":
			a.active = (a.active + 1) % len(a.tabs)
			return a, nil
		case "shift+tab":
			a.active = (a.active - 1 + len(a.tabs)) % len(a.tabs)
			return a, nil
		}
	}

	if a.inSetup {
		var cmd tea.Cmd
		a.setup, cmd = a.setup.Update(msg)
		return a, cmd
	}

	var cmd tea.Cmd
	switch a.active {
	case 0:
		a.collections, cmd = a.collections.Update(msg)
	case 1:
		a.nlModels, cmd = a.nlModels.Update(msg)
	case 2:
		a.curations, cmd = a.curations.Update(msg)
	case 3:
		a.conversations, cmd = a.conversations.Update(msg)
	case 4:
		a.settings, cmd = a.settings.Update(msg)
	}
	return a, cmd
}

func (a App) View() string {
	if a.inSetup {
		return a.setup.View()
	}
	bar := components.TabBar{Titles: a.tabs, Active: a.active}
	bar.Style.Active = TabActive
	bar.Style.Inactive = TabInactive
	body := ""
	switch a.active {
	case 0:
		body = a.collections.View()
	case 1:
		body = a.nlModels.View()
	case 2:
		body = a.curations.View()
	case 3:
		body = a.conversations.View()
	case 4:
		body = a.settings.View()
	}
	return bar.View() + "\n\n" + body
}
