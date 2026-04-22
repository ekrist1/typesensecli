package components

import tea "github.com/charmbracelet/bubbletea"

type ConfirmResultMsg struct {
	Confirmed bool
	Tag       string // so callers can disambiguate which confirm returned
}

type Confirm struct {
	Prompt string
	Tag    string
}

func NewConfirm(prompt, tag string) Confirm { return Confirm{Prompt: prompt, Tag: tag} }

func (c Confirm) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			return func() tea.Msg { return ConfirmResultMsg{Confirmed: true, Tag: c.Tag} }
		case "n", "N", "esc":
			return func() tea.Msg { return ConfirmResultMsg{Confirmed: false, Tag: c.Tag} }
		}
	}
	return nil
}

func (c Confirm) View() string {
	return c.Prompt + "\n\n[y] yes   [n] no"
}
