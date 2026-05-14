package screens

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

type synonymCreateMode int

const (
	synonymCreateWizard synonymCreateMode = iota
	synonymCreateRaw
)

type synonymLinkState struct {
	setName     string
	collections []string
	index       int
	errors      []string
}

// Synonyms wraps the generic Resource screen for synonym sets. New synonym
// sets are saved with PUT /synonym_sets/{name}, then linked to collections by
// patching each collection's `synonym_sets` list.
type Synonyms struct {
	inner Resource

	createMode synonymCreateMode

	namePrompt *textinput.Model
	linkPrompt *textinput.Model
	wizard     *synonymWizard

	promptErr          string
	pendingName        string
	pendingCollections []string
	pendingLink        *synonymLinkState
	linking            *synonymLinkState
}

func NewSynonyms(c *client.Client, w, h int) Synonyms {
	inner := NewResource(c, ResourceStrategy{
		TabName:          "Synonyms",
		Template:         templates.Synonym,
		List:             client.ListSynonymSets,
		Create:           nil,
		Update:           client.UpsertSynonymSet,
		Delete:           client.DeleteSynonymSet,
		CreateNamePrompt: true,
		ExtractItems:     synonymsExtractItems,
		ExtractDetail:    synonymsExtractDetail,
	}, w, h)
	return Synonyms{inner: inner}
}

func (s *Synonyms) SetSize(w, h int) { s.inner.SetSize(w, h) }

func (s Synonyms) Init() tea.Cmd { return s.inner.Init() }

func (s Synonyms) HasModal() bool {
	return s.namePrompt != nil || s.linkPrompt != nil || s.wizard != nil || s.inner.HasModal()
}

func (s Synonyms) Update(msg tea.Msg) (Synonyms, tea.Cmd) {
	if s.namePrompt != nil {
		return s.updateNamePrompt(msg)
	}
	if s.linkPrompt != nil {
		return s.updateLinkPrompt(msg)
	}
	if s.wizard != nil {
		return s.updateWizard(msg)
	}

	switch m := msg.(type) {
	case api.SuccessMsg:
		switch {
		case s.pendingLink != nil && m.Tag == s.inner.tag("save"):
			var cmd tea.Cmd
			s.inner, cmd = s.inner.Update(msg)
			s.linking = s.pendingLink
			s.pendingLink = nil
			s.inner.status = fmt.Sprintf("linking %q to %s", s.linking.setName, strings.Join(s.linking.collections, ", "))
			return s, tea.Batch(cmd, s.nextLinkFetch())
		case s.isLinkGetTag(m.Tag):
			return s.handleLinkGetSuccess(m)
		case s.isLinkPatchTag(m.Tag):
			return s.handleLinkPatchSuccess(m)
		}
	case api.ErrorMsg:
		switch {
		case m.Tag == s.inner.tag("save"):
			s.pendingLink = nil
			s.pendingName = ""
			s.pendingCollections = nil
		case s.isLinkGetTag(m.Tag):
			return s.handleLinkError(m, "load collection")
		case s.isLinkPatchTag(m.Tag):
			return s.handleLinkError(m, "patch collection")
		}
	case components.JSONEditorSubmitMsg:
		if s.inner.editor != nil && s.inner.pendingName != "" && !s.inner.isEditing && len(s.pendingCollections) > 0 {
			s.pendingLink = &synonymLinkState{
				setName:     s.inner.pendingName,
				collections: append([]string(nil), s.pendingCollections...),
			}
		}
	case components.JSONEditorCancelMsg:
		if s.inner.editor != nil && s.inner.pendingName != "" && !s.inner.isEditing {
			s.pendingName = ""
			s.pendingCollections = nil
			s.pendingLink = nil
		}
	case tea.KeyMsg:
		if s.inner.namePrompt == nil && s.inner.editor == nil && s.inner.confirm == nil {
			switch m.String() {
			case "n":
				return s, s.openCreateNamePrompt(synonymCreateWizard)
			case "N":
				return s, s.openCreateNamePrompt(synonymCreateRaw)
			case "e":
				return s, s.openWizardEdit()
			case "E":
				return s, s.openRawEdit()
			}
		}
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s Synonyms) View() string {
	switch {
	case s.namePrompt != nil:
		body := "Synonym set name:\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		if s.createMode == synonymCreateRaw {
			body = "Synonym set name (raw JSON):\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		}
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.linkPrompt != nil:
		body := "Collections to link this synonym set to:\n\n" + s.linkPrompt.View() +
			"\n\nComma-separated names · Enter continue · Esc cancel"
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.wizard != nil:
		return s.wizard.View()
	default:
		view := s.inner.View()
		return strings.Replace(view,
			"/ filter · n new · e edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			"/ filter · n wizard new · N raw new · e wizard edit · E raw edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			1,
		)
	}
}

func (s Synonyms) updateNamePrompt(msg tea.Msg) (Synonyms, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			s.clearCreateState()
			return s, nil
		case "enter":
			name := strings.TrimSpace(s.namePrompt.Value())
			if name == "" {
				s.promptErr = "synonym set name is required"
				return s, nil
			}
			s.pendingName = name
			s.promptErr = ""
			ti := textinput.New()
			ti.Placeholder = "products, brands"
			ti.Focus()
			s.namePrompt = nil
			s.linkPrompt = &ti
			return s, textinput.Blink
		}
	}
	updated, cmd := s.namePrompt.Update(msg)
	s.namePrompt = &updated
	return s, cmd
}

func (s Synonyms) updateLinkPrompt(msg tea.Msg) (Synonyms, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			s.clearCreateState()
			return s, nil
		case "enter":
			collections := parseCollectionNames(s.linkPrompt.Value())
			if len(collections) == 0 {
				s.promptErr = "at least one collection name is required"
				return s, nil
			}
			s.pendingCollections = collections
			s.promptErr = ""
			s.linkPrompt = nil
			switch s.createMode {
			case synonymCreateWizard:
				wiz := newSynonymWizard(s.pendingName, s.inner.width, s.inner.height)
				s.wizard = &wiz
				return s, textinput.Blink
			case synonymCreateRaw:
				ed := components.NewJSONEditor("New Synonyms: "+s.pendingName, s.inner.strat.Template, s.inner.width, s.inner.height)
				s.inner.pendingName = s.pendingName
				s.inner.editor = &ed
				s.inner.isEditing = false
				return s, nil
			default:
				s.clearCreateState()
				return s, nil
			}
		}
	}
	updated, cmd := s.linkPrompt.Update(msg)
	s.linkPrompt = &updated
	return s, cmd
}

func (s Synonyms) updateWizard(msg tea.Msg) (Synonyms, tea.Cmd) {
	updated, cmd := s.wizard.Update(msg)
	s.wizard = &updated
	if body := s.wizard.Submitted(); body != nil {
		name := s.wizard.Name()
		s.wizard = nil
		if len(s.pendingCollections) > 0 {
			s.pendingLink = &synonymLinkState{
				setName:     name,
				collections: append([]string(nil), s.pendingCollections...),
			}
		}
		s.pendingName = ""
		s.pendingCollections = nil
		m, p := client.UpsertSynonymSet(name)
		return s, api.DoRequest(s.inner.c, s.inner.tag("save"), m, p, body)
	}
	if s.wizard.Cancelled() {
		s.wizard = nil
		s.clearCreateState()
	}
	return s, cmd
}

func (s *Synonyms) openCreateNamePrompt(mode synonymCreateMode) tea.Cmd {
	s.clearCreateState()
	s.createMode = mode
	ti := textinput.New()
	ti.Placeholder = "synonym set name"
	ti.Focus()
	s.namePrompt = &ti
	return textinput.Blink
}

func (s *Synonyms) openWizardEdit() tea.Cmd {
	it, ok := s.inner.list.SelectedItem().(resourceItem)
	if !ok {
		return nil
	}
	body := s.inner.strat.ExtractDetail(s.inner.rawList, it.id)
	if body == nil {
		s.inner.status = "unable to load selected synonym"
		return nil
	}
	wiz, err := newSynonymWizardFromBody(it.id, body, s.inner.width, s.inner.height)
	if err != nil {
		s.inner.status = err.Error() + " - use E for raw JSON edit"
		return nil
	}
	s.wizard = &wiz
	return nil
}

func (s *Synonyms) openRawEdit() tea.Cmd {
	it, ok := s.inner.list.SelectedItem().(resourceItem)
	if !ok {
		return nil
	}
	body := s.inner.strat.ExtractDetail(s.inner.rawList, it.id)
	if body == nil {
		body = s.inner.strat.Template
	}
	ed := components.NewJSONEditor("Edit "+it.id, body, s.inner.width, s.inner.height)
	s.inner.editor = &ed
	s.inner.isEditing = true
	return nil
}

func (s *Synonyms) clearCreateState() {
	s.namePrompt = nil
	s.linkPrompt = nil
	s.wizard = nil
	s.promptErr = ""
	s.pendingName = ""
	s.pendingCollections = nil
	s.pendingLink = nil
}

func (s Synonyms) linkGetTag(collection string) string {
	return s.inner.tag("link:get:" + collection)
}

func (s Synonyms) linkPatchTag(collection string) string {
	return s.inner.tag("link:patch:" + collection)
}

func (s Synonyms) isLinkGetTag(tag string) bool {
	return strings.HasPrefix(tag, s.inner.tag("link:get:"))
}

func (s Synonyms) isLinkPatchTag(tag string) bool {
	return strings.HasPrefix(tag, s.inner.tag("link:patch:"))
}

func (s *Synonyms) nextLinkFetch() tea.Cmd {
	if s.linking == nil || s.linking.index >= len(s.linking.collections) {
		s.finishLinking()
		return nil
	}
	collection := s.linking.collections[s.linking.index]
	s.inner.status = fmt.Sprintf("linking %q to %s (%d/%d)", s.linking.setName, collection, s.linking.index+1, len(s.linking.collections))
	m, p := client.GetCollection(collection)
	return api.DoRequest(s.inner.c, s.linkGetTag(collection), m, p, nil)
}

func (s Synonyms) handleLinkGetSuccess(msg api.SuccessMsg) (Synonyms, tea.Cmd) {
	if s.linking == nil {
		return s, nil
	}
	collection := strings.TrimPrefix(msg.Tag, s.inner.tag("link:get:"))
	var payload collectionSchemaPayload
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		s.linking.errors = append(s.linking.errors, fmt.Sprintf("%s: parse error: %v", collection, err))
		s.linking.index++
		return s, s.nextLinkFetch()
	}

	synonymSets := append([]string(nil), payload.SynonymSets...)
	if !containsString(synonymSets, s.linking.setName) {
		synonymSets = append(synonymSets, s.linking.setName)
	}
	body, _ := json.Marshal(map[string]any{"synonym_sets": synonymSets})
	m, p := client.PatchCollection(collection)
	return s, api.DoRequest(s.inner.c, s.linkPatchTag(collection), m, p, body)
}

func (s Synonyms) handleLinkPatchSuccess(msg api.SuccessMsg) (Synonyms, tea.Cmd) {
	if s.linking == nil {
		return s, nil
	}
	s.linking.index++
	return s, s.nextLinkFetch()
}

func (s Synonyms) handleLinkError(msg api.ErrorMsg, op string) (Synonyms, tea.Cmd) {
	if s.linking == nil {
		return s, nil
	}
	var detail string
	if msg.Err != nil {
		detail = msg.Err.Error()
	} else {
		detail = fmt.Sprintf("HTTP %d: %s", msg.Status, string(msg.Body))
	}

	collection := ""
	if s.isLinkGetTag(msg.Tag) {
		collection = strings.TrimPrefix(msg.Tag, s.inner.tag("link:get:"))
	} else if s.isLinkPatchTag(msg.Tag) {
		collection = strings.TrimPrefix(msg.Tag, s.inner.tag("link:patch:"))
	}
	s.linking.errors = append(s.linking.errors, fmt.Sprintf("%s: %s failed: %s", collection, op, detail))
	s.linking.index++
	return s, s.nextLinkFetch()
}

func (s *Synonyms) finishLinking() {
	if s.linking == nil {
		return
	}
	count := len(s.linking.collections)
	name := s.linking.setName
	if len(s.linking.errors) == 0 {
		s.inner.status = fmt.Sprintf("linked %q to %d collection(s)", name, count)
	} else {
		s.inner.status = fmt.Sprintf("linked %q with %d issue(s): %s", name, len(s.linking.errors), strings.Join(s.linking.errors, "; "))
	}
	s.linking = nil
	s.pendingName = ""
	s.pendingCollections = nil
}

func synonymsExtractItems(body []byte) ([]string, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(arr))
	for _, m := range arr {
		if name, ok := m["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

func synonymsExtractDetail(body []byte, name string) []byte {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil
	}
	for _, m := range arr {
		if m["name"] == name {
			b, _ := json.MarshalIndent(m, "", "  ")
			return b
		}
	}
	return nil
}
