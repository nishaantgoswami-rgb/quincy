package models

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	OAuthConfirmDialogID dialogs.DialogID = "oauth-confirm"
)

type oauthConfirmDialog struct {
	width        int
	wWidth       int
	wHeight      int
	providerName string
	keyMap       KeyMap
	selected     bool // true = Yes, false = No
}

func NewOAuthConfirmDialog(providerName string) dialogs.DialogModel {
	keyMap := DefaultKeyMap()
	
	return &oauthConfirmDialog{
		providerName: providerName,
		keyMap:       keyMap,
		width:        60,
		selected:     true, // Default to Yes
	}
}

func (o *oauthConfirmDialog) Init() tea.Cmd {
	return nil
}

func (o *oauthConfirmDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.wWidth = msg.Width
		o.wHeight = msg.Height
		return o, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, o.keyMap.Select):
			// Enter key - confirm the selected option
			if o.selected {
				return o, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(OAuthConfirmMsg{Action: OAuthConfirm}),
				)
			} else {
				return o, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(OAuthConfirmMsg{Action: OAuthCancel}),
				)
			}
		case key.Matches(msg, o.keyMap.Tab):
			// Toggle between Yes and No
			o.selected = !o.selected
			return o, nil
		case key.Matches(msg, o.keyMap.Close):
			// Escape key - cancel and go back
			return o, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(OAuthConfirmMsg{Action: OAuthCancel}),
			)
		}
	}
	return o, nil
}

func (o *oauthConfirmDialog) View() string {
	t := styles.CurrentTheme()
	
	title := t.S().Base.Foreground(t.Primary).Render("Confirm OAuth Authentication")
	
	message := t.S().Base.Foreground(t.FgMuted).Render(
		"You've selected " + o.providerName + " which requires OAuth authentication.",
	)
	
	question := t.S().Base.Foreground(t.FgMuted).Render(
		"Do you want to proceed with OAuth authentication?",
	)
	
	// Create buttons with proper selection state
	var yesButton, noButton string
	if o.selected {
		yesButton = core.SelectableButton(core.ButtonOpts{
			Text:           "Yes, proceed",
			UnderlineIndex: 0,
			Selected:       true,
		})
		noButton = core.SelectableButton(core.ButtonOpts{
			Text:           "No, go back",
			UnderlineIndex: 0,
			Selected:       false,
		})
	} else {
		yesButton = core.SelectableButton(core.ButtonOpts{
			Text:           "Yes, proceed",
			UnderlineIndex: 0,
			Selected:       false,
		})
		noButton = core.SelectableButton(core.ButtonOpts{
			Text:           "No, go back",
			UnderlineIndex: 0,
			Selected:       true,
		})
	}
	
	buttons := lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		question,
		"",
		buttons,
	)
	
	dialog := t.S().Base.
		Width(o.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1).
		Render(content)
	
	return dialog
}

func (o *oauthConfirmDialog) Cursor() *tea.Cursor {
	return nil
}

func (o *oauthConfirmDialog) Position() (int, int) {
	row := o.wHeight/2 - 5
	col := o.wWidth/2 - o.width/2
	return row, col
}

func (o *oauthConfirmDialog) ID() dialogs.DialogID {
	return OAuthConfirmDialogID
}