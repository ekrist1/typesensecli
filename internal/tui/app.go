package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/config"
	"clisense/internal/tui/api"
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
	stopwords     screens.Resource
	presets       screens.Resource
	conversations screens.Conversations
	keys          screens.Keys
	analytics     screens.Resource
	settings      screens.Settings

	tabs []string
}

func NewApp(cfg config.Config, cfgPath string, inSetup bool) App {
	a := App{
		cfg:     cfg,
		cfgPath: cfgPath,
		inSetup: inSetup,
		setup:   screens.NewSetup(cfg),
		tabs:    []string{"Collections", "Aliases", "NL Models", "Curations", "Synonyms", "Stopwords", "Presets", "Conversations", "API Keys", "Analytics", "Settings"},
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
	a.stopwords = screens.NewStopwords(a.c, a.width, a.height-3)
	a.presets = screens.NewPresets(a.c, a.width, a.height-3)
	a.conversations = screens.NewConversations(a.c, a.width, a.height-3)
	a.keys = screens.NewKeys(a.c, a.width, a.height-3)
	a.analytics = screens.NewAnalytics(a.c, a.width, a.height-3)
	a.settings = screens.NewSettings(a.cfg)
}

func (a App) settingsTabIndex() int { return len(a.tabs) - 1 }

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
		return a.stopwords.Init()
	case 6:
		return a.presets.Init()
	case 7:
		return a.conversations.Init()
	case 8:
		return a.keys.Init()
	case 9:
		return a.analytics.Init()
	case 10:
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
			a.stopwords.SetSize(a.width, a.height-3)
			a.presets.SetSize(a.width, a.height-3)
			a.conversations.SetSize(a.width, a.height-3)
			a.keys.SetSize(a.width, a.height-3)
			a.analytics.SetSize(a.width, a.height-3)
		}
		return a, nil
	case screens.SetupDoneMsg:
		wasSetup := a.inSetup
		a.cfg = m.Cfg
		if err := config.Save(a.cfgPath, a.cfg); err != nil {
			if a.inSetup {
				a.setup.SetError("save failed: " + err.Error())
			} else {
				a.settings.SetError("save failed: " + err.Error())
			}
			return a, nil
		}
		a.c = client.New(a.cfg.URL, a.cfg.APIKey)
		a.inSetup = false
		a.buildTabs()
		if wasSetup {
			a.active = 0
			return a, a.collections.Init()
		}
		return a, a.activateTab(a.active)
	case api.SuccessMsg:
		if !a.inSetup {
			return a, a.routeTaggedAPIMessage(m.Tag, msg)
		}
	case api.ErrorMsg:
		if !a.inSetup {
			return a, a.routeTaggedAPIMessage(m.Tag, msg)
		}
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
		// Settings is a form, so keep Tab/digits/q available for the inputs.
		// Esc returns to Collections.
		if a.active == a.settingsTabIndex() {
			switch m.String() {
			case "esc":
				return a, a.activateTab(0)
			}
		}
		// Other globals (q, 1-9, tab, shift+tab) only fire when the active
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
			case "8":
				return a, a.activateTab(7)
			case "9":
				return a, a.activateTab(8)
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
		a.stopwords, cmd = a.stopwords.Update(msg)
	case 6:
		a.presets, cmd = a.presets.Update(msg)
	case 7:
		a.conversations, cmd = a.conversations.Update(msg)
	case 8:
		a.keys, cmd = a.keys.Update(msg)
	case 9:
		a.analytics, cmd = a.analytics.Update(msg)
	case 10:
		a.settings, cmd = a.settings.Update(msg)
	}
	return a, cmd
}

func (a *App) routeTaggedAPIMessage(tag string, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case tag == "collections" || tag == "collection-delete" ||
		strings.HasPrefix(tag, "collection-detail:") ||
		strings.HasPrefix(tag, "collection:search:"):
		a.collections, cmd = a.collections.Update(msg)
	case strings.HasPrefix(tag, "Aliases:"):
		a.aliases, cmd = a.aliases.Update(msg)
	case strings.HasPrefix(tag, "NL Models:"):
		a.nlModels, cmd = a.nlModels.Update(msg)
	case strings.HasPrefix(tag, "Curations:"):
		a.curations, cmd = a.curations.Update(msg)
	case strings.HasPrefix(tag, "Synonyms:"):
		a.synonyms, cmd = a.synonyms.Update(msg)
	case strings.HasPrefix(tag, "Stopwords:"):
		a.stopwords, cmd = a.stopwords.Update(msg)
	case strings.HasPrefix(tag, "Presets:"):
		a.presets, cmd = a.presets.Update(msg)
	case strings.HasPrefix(tag, "Conversations:") || tag == "conversations:test":
		a.conversations, cmd = a.conversations.Update(msg)
	case strings.HasPrefix(tag, "API Keys:"):
		a.keys, cmd = a.keys.Update(msg)
	case strings.HasPrefix(tag, "Analytics:"):
		a.analytics, cmd = a.analytics.Update(msg)
	case strings.HasPrefix(tag, "docpicker:"):
		switch a.active {
		case 3:
			a.curations, cmd = a.curations.Update(msg)
		case 4:
			a.synonyms, cmd = a.synonyms.Update(msg)
		}
	}
	return cmd
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
		return a.stopwords.HasModal()
	case 6:
		return a.presets.HasModal()
	case 7:
		return a.conversations.HasModal()
	case 8:
		return a.keys.HasModal()
	case 9:
		return a.analytics.HasModal()
	case 10:
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
		body = a.stopwords.View()
	case 6:
		body = a.presets.View()
	case 7:
		body = a.conversations.View()
	case 8:
		body = a.keys.View()
	case 9:
		body = a.analytics.View()
	case 10:
		body = a.settings.View()
	}
	return bar.View() + "\n\n" + body
}
