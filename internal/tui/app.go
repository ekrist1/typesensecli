package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/config"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
	"clisense/internal/tui/screens"
	"clisense/internal/tui/theme"
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
	cluster       screens.Cluster
	settings      screens.Settings

	tabIDs []string
	tabs   []string
}

func NewApp(cfg config.Config, cfgPath string, inSetup bool) App {
	a := App{
		cfg:     cfg,
		cfgPath: cfgPath,
		inSetup: inSetup,
		setup:   screens.NewSetup(cfg),
		width:   100,
		height:  30,
	}
	a.configureTabs()
	if !inSetup {
		a.c = client.New(cfg.URL, cfg.APIKey)
		a.buildTabs()
	}
	return a
}

var tabTitles = map[string]string{
	"collections":   "Collections",
	"aliases":       "Aliases",
	"nl_models":     "NL Models",
	"curations":     "Curations",
	"synonyms":      "Synonyms",
	"stopwords":     "Stopwords",
	"presets":       "Presets",
	"conversations": "Conversations",
	"api_keys":      "API Keys",
	"analytics":     "Analytics",
	"cluster":       "Cluster",
	"settings":      "Settings",
}

func (a *App) configureTabs() {
	a.tabIDs = nil
	a.tabs = nil
	for _, id := range a.cfg.TabOrder() {
		title, ok := tabTitles[id]
		if !ok {
			continue
		}
		a.tabIDs = append(a.tabIDs, id)
		a.tabs = append(a.tabs, title)
	}
	if len(a.tabIDs) == 0 {
		a.tabIDs = []string{"settings"}
		a.tabs = []string{tabTitles["settings"]}
	}
	if a.active >= len(a.tabIDs) {
		a.active = len(a.tabIDs) - 1
	}
}

func (a *App) buildTabs() {
	a.collections = screens.NewCollections(a.c, a.width, a.height-4)
	a.aliases = screens.NewAliases(a.c, a.width, a.height-4)
	a.nlModels = screens.NewNLModels(a.c, a.width, a.height-4)
	a.curations = screens.NewCurations(a.c, a.width, a.height-4)
	a.synonyms = screens.NewSynonyms(a.c, a.width, a.height-4)
	a.stopwords = screens.NewStopwords(a.c, a.width, a.height-4)
	a.presets = screens.NewPresets(a.c, a.width, a.height-4)
	a.conversations = screens.NewConversations(a.c, a.width, a.height-4)
	a.keys = screens.NewKeys(a.c, a.width, a.height-4)
	a.analytics = screens.NewAnalytics(a.c, a.width, a.height-4)
	a.cluster = screens.NewCluster(a.c, a.width, a.height-4)
	a.settings = screens.NewSettings(a.cfg)
}

func (a App) settingsTabIndex() int {
	for i, id := range a.tabIDs {
		if id == "settings" {
			return i
		}
	}
	return len(a.tabIDs) - 1
}

func (a App) activeTabID() string {
	if a.active < 0 || a.active >= len(a.tabIDs) {
		return ""
	}
	return a.tabIDs[a.active]
}

func (a App) Init() tea.Cmd {
	if a.inSetup {
		return a.setup.Init()
	}
	return a.activateTab(0)
}

func (a *App) activateTab(index int) tea.Cmd {
	if index < 0 || index >= len(a.tabIDs) {
		return nil
	}
	a.active = index
	switch a.activeTabID() {
	case "collections":
		return a.collections.Init()
	case "aliases":
		return a.aliases.Init()
	case "nl_models":
		return a.nlModels.Init()
	case "curations":
		return a.curations.Init()
	case "synonyms":
		return a.synonyms.Init()
	case "stopwords":
		return a.stopwords.Init()
	case "presets":
		return a.presets.Init()
	case "conversations":
		return a.conversations.Init()
	case "api_keys":
		return a.keys.Init()
	case "analytics":
		return a.analytics.Init()
	case "cluster":
		return a.cluster.Init()
	case "settings":
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
			a.collections.SetSize(a.width, a.height-4)
			a.aliases.SetSize(a.width, a.height-4)
			a.nlModels.SetSize(a.width, a.height-4)
			a.curations.SetSize(a.width, a.height-4)
			a.synonyms.SetSize(a.width, a.height-4)
			a.stopwords.SetSize(a.width, a.height-4)
			a.presets.SetSize(a.width, a.height-4)
			a.conversations.SetSize(a.width, a.height-4)
			a.keys.SetSize(a.width, a.height-4)
			a.analytics.SetSize(a.width, a.height-4)
			a.cluster.SetSize(a.width, a.height-4)
		}
		return a, nil
	case screens.SetupDoneMsg:
		wasSetup := a.inSetup
		wasSettings := !a.inSetup && a.activeTabID() == "settings"
		a.cfg = m.Cfg
		if err := config.Save(a.cfgPath, a.cfg); err != nil {
			if a.inSetup {
				a.setup.SetError("save failed: " + err.Error())
			} else {
				a.settings.SetError("save failed: " + err.Error())
			}
			return a, nil
		}
		a.configureTabs()
		a.c = client.New(a.cfg.URL, a.cfg.APIKey)
		a.inSetup = false
		a.buildTabs()
		if wasSetup {
			a.active = 0
			return a, a.activateTab(0)
		}
		if wasSettings {
			return a, a.activateTab(0)
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
		// Ctrl+C always quits, including the first-run setup form.
		if m.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.inSetup {
			var cmd tea.Cmd
			a.setup, cmd = a.setup.Update(msg)
			return a, cmd
		}
		// Settings is a form, so keep Tab/digits/q available for the inputs.
		// Esc returns to Collections.
		if a.activeTabID() == "settings" {
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
				return a, a.activateTab((a.active + 1) % len(a.tabIDs))
			case "shift+tab":
				return a, a.activateTab((a.active - 1 + len(a.tabIDs)) % len(a.tabIDs))
			}
		}
	}

	if a.inSetup {
		var cmd tea.Cmd
		a.setup, cmd = a.setup.Update(msg)
		return a, cmd
	}

	var cmd tea.Cmd
	switch a.activeTabID() {
	case "collections":
		a.collections, cmd = a.collections.Update(msg)
	case "aliases":
		a.aliases, cmd = a.aliases.Update(msg)
	case "nl_models":
		a.nlModels, cmd = a.nlModels.Update(msg)
	case "curations":
		a.curations, cmd = a.curations.Update(msg)
	case "synonyms":
		a.synonyms, cmd = a.synonyms.Update(msg)
	case "stopwords":
		a.stopwords, cmd = a.stopwords.Update(msg)
	case "presets":
		a.presets, cmd = a.presets.Update(msg)
	case "conversations":
		a.conversations, cmd = a.conversations.Update(msg)
	case "api_keys":
		a.keys, cmd = a.keys.Update(msg)
	case "analytics":
		a.analytics, cmd = a.analytics.Update(msg)
	case "cluster":
		a.cluster, cmd = a.cluster.Update(msg)
	case "settings":
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
	case strings.HasPrefix(tag, "Cluster:"):
		a.cluster, cmd = a.cluster.Update(msg)
	case strings.HasPrefix(tag, "docpicker:"):
		switch a.activeTabID() {
		case "curations":
			a.curations, cmd = a.curations.Update(msg)
		case "synonyms":
			a.synonyms, cmd = a.synonyms.Update(msg)
		}
	}
	return cmd
}

// activeHasModal reports whether the currently-visible screen has a modal,
// wizard, or in-screen input that should receive keys like "q", "tab", or
// digits before the global handlers consume them.
func (a App) activeHasModal() bool {
	switch a.activeTabID() {
	case "collections":
		return a.collections.HasModal()
	case "aliases":
		return a.aliases.HasModal()
	case "nl_models":
		return a.nlModels.HasModal()
	case "curations":
		return a.curations.HasModal()
	case "synonyms":
		return a.synonyms.HasModal()
	case "stopwords":
		return a.stopwords.HasModal()
	case "presets":
		return a.presets.HasModal()
	case "conversations":
		return a.conversations.HasModal()
	case "api_keys":
		return a.keys.HasModal()
	case "analytics":
		return a.analytics.HasModal()
	case "cluster":
		return a.cluster.HasModal()
	case "settings":
		// Settings is a form — always treat as modal.
		return true
	}
	return false
}

func (a App) View() string {
	if a.inSetup {
		return a.setup.View()
	}
	titles := make([]string, len(a.tabs))
	for i, t := range a.tabs {
		if i >= 9 {
			titles[i] = t
			continue
		}
		digit := strconv.Itoa(i + 1)
		if i == a.active {
			titles[i] = digit + " " + t
		} else {
			titles[i] = theme.TabNumber.Render(digit) + " " + t
		}
	}
	bar := components.TabBar{Titles: titles, Active: a.active, Width: a.width}
	bar.Style.Active = theme.TabActive
	bar.Style.Inactive = theme.TabInactive
	header := theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — "+a.cfg.URL)
	body := ""
	switch a.activeTabID() {
	case "collections":
		body = a.collections.View()
	case "aliases":
		body = a.aliases.View()
	case "nl_models":
		body = a.nlModels.View()
	case "curations":
		body = a.curations.View()
	case "synonyms":
		body = a.synonyms.View()
	case "stopwords":
		body = a.stopwords.View()
	case "presets":
		body = a.presets.View()
	case "conversations":
		body = a.conversations.View()
	case "api_keys":
		body = a.keys.View()
	case "analytics":
		body = a.analytics.View()
	case "cluster":
		body = a.cluster.View()
	case "settings":
		body = a.settings.View()
	}
	return header + "\n" + bar.View() + "\n\n" + body
}
