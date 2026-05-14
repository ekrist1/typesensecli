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
	aliases       screens.Resource
	nlModels      screens.Resource
	curations     screens.Curations
	synonyms      screens.Synonyms
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
		tabs:    []string{"Collections", "Aliases", "NL Models", "Curations", "Synonyms", "Conversations", "Settings"},
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
	a.aliases = screens.NewAliases(a.c, a.width, a.height-3)
	a.nlModels = screens.NewNLModels(a.c, a.width, a.height-3)
	a.curations = screens.NewCurations(a.c, a.width, a.height-3)
	a.synonyms = screens.NewSynonyms(a.c, a.width, a.height-3)
	a.conversations = screens.NewConversations(a.c, a.width, a.height-3)
	a.settings = screens.NewSettings(a.cfg)
}

func (a App) Init() tea.Cmd {
	if a.inSetup {
		return a.setup.Init()
	}
	return a.collections.Init()
}

func (a *App) activateTab(index int) tea.Cmd {
	a.active = index
	switch index {
	case 0:
		return a.collections.Init()
	case 1:
		return a.aliases.Init()
	case 2:
		return a.nlModels.Init()
	case 3:
		return a.curations.Init()
	case 4:
		return a.synonyms.Init()
	case 5:
		return a.conversations.Init()
	case 6:
		return a.settings.Init()
	default:
		return nil
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		if !a.inSetup {
			a.collections.SetSize(a.width, a.height-3)
			a.aliases.SetSize(a.width, a.height-3)
			a.nlModels.SetSize(a.width, a.height-3)
			a.curations.SetSize(a.width, a.height-3)
			a.synonyms.SetSize(a.width, a.height-3)
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
		// Ctrl+C always quits.
		if m.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// Settings is a form so it's treated as modal (keeps "q" and digits
		// typable into the URL/API key fields) — but then tab/shift+tab
		// don't reach the global tab-cycle handler, leaving the user stuck.
		// Let tab/shift+tab escape Settings specifically.
		if a.active == 6 {
			switch m.String() {
			case "tab":
				return a, a.activateTab((a.active + 1) % len(a.tabs))
			case "shift+tab":
				return a, a.activateTab((a.active - 1 + len(a.tabs)) % len(a.tabs))
			}
		}
		// Other globals (q, 1-7, tab, shift+tab) only fire when the active
		// screen has no modal open — otherwise typing "q" in an input would
		// quit the app and "tab" would switch tabs instead of cycling fields.
		if !a.activeHasModal() {
			switch m.String() {
			case "q":
				return a, tea.Quit
			case "1":
				return a, a.activateTab(0)
			case "2":
				return a, a.activateTab(1)
			case "3":
				return a, a.activateTab(2)
			case "4":
				return a, a.activateTab(3)
			case "5":
				return a, a.activateTab(4)
			case "6":
				return a, a.activateTab(5)
			case "7":
				return a, a.activateTab(6)
			case "tab":
				return a, a.activateTab((a.active + 1) % len(a.tabs))
			case "shift+tab":
				return a, a.activateTab((a.active - 1 + len(a.tabs)) % len(a.tabs))
			}
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
		a.aliases, cmd = a.aliases.Update(msg)
	case 2:
		a.nlModels, cmd = a.nlModels.Update(msg)
	case 3:
		a.curations, cmd = a.curations.Update(msg)
	case 4:
		a.synonyms, cmd = a.synonyms.Update(msg)
	case 5:
		a.conversations, cmd = a.conversations.Update(msg)
	case 6:
		a.settings, cmd = a.settings.Update(msg)
	}
	return a, cmd
}

// activeHasModal reports whether the currently-visible screen has a modal,
// wizard, or in-screen input that should receive keys like "q", "tab", or
// digits before the global handlers consume them.
func (a App) activeHasModal() bool {
	switch a.active {
	case 0:
		return a.collections.HasModal()
	case 1:
		return a.aliases.HasModal()
	case 2:
		return a.nlModels.HasModal()
	case 3:
		return a.curations.HasModal()
	case 4:
		return a.synonyms.HasModal()
	case 5:
		return a.conversations.HasModal()
	case 6:
		// Settings is a form — always treat as modal.
		return true
	}
	return false
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
		body = a.aliases.View()
	case 2:
		body = a.nlModels.View()
	case 3:
		body = a.curations.View()
	case 4:
		body = a.synonyms.View()
	case 5:
		body = a.conversations.View()
	case 6:
		body = a.settings.View()
	}
	return bar.View() + "\n\n" + body
}
