package oauth

import (
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	OAuthDialogID dialogs.DialogID = "oauth"
)

type OAuthState int

const (
	OAuthStateInitial OAuthState = iota
	OAuthStateRedirecting
	OAuthStateWaiting
	OAuthStateExchanging
	OAuthStateSuccess
	OAuthStateError
)

type OAuthStateChangeMsg struct {
	State OAuthState
	Error error
}

type OAuthDialog interface {
	dialogs.DialogModel
	dialogs.CloseCallback
}

// OAuthDialog Interface implementation verification
var _ OAuthDialog = (*oauthDialogCmp)(nil)

type oauthDialogCmp struct {
	width        int
	wWidth       int
	wHeight      int
	state        OAuthState
	spinner      spinner.Model
	providerName string
	errorMessage string
	successMessage string
	authCode     string
}

func NewOAuthDialogCmp(providerName string) OAuthDialog {
	t := styles.CurrentTheme()

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.S().Base.Foreground(t.Green)),
	)

	return &oauthDialogCmp{
		state:        OAuthStateInitial,
		spinner:      s,
		providerName: providerName,
		width:        60,
	}
}

func (o *oauthDialogCmp) Init() tea.Cmd {
	return o.spinner.Tick
}

func (o *oauthDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.wWidth = msg.Width
		o.wHeight = msg.Height
		return o, nil
	case spinner.TickMsg:
		if o.state == OAuthStateRedirecting || o.state == OAuthStateWaiting || o.state == OAuthStateExchanging {
			var cmd tea.Cmd
			o.spinner, cmd = o.spinner.Update(msg)
			return o, cmd
		}
		return o, nil
	case OAuthStateChangeMsg:
		o.state = msg.State
		if msg.Error != nil {
			o.errorMessage = msg.Error.Error()
		}
		var cmd tea.Cmd
		if msg.State == OAuthStateRedirecting || msg.State == OAuthStateWaiting || msg.State == OAuthStateExchanging {
			cmd = o.spinner.Tick
		}
		return o, cmd
	}
	return o, nil
}

func (o *oauthDialogCmp) View() string {
	t := styles.CurrentTheme()

	var content string
	switch o.state {
	case OAuthStateInitial:
		content = o.renderInitialView()
	case OAuthStateRedirecting:
		content = o.renderRedirectingView()
	case OAuthStateWaiting:
		content = o.renderWaitingView()
	case OAuthStateExchanging:
		content = o.renderExchangingView()
	case OAuthStateSuccess:
		content = o.renderSuccessView()
	case OAuthStateError:
		content = o.renderErrorView()
	}

	dialog := t.S().Base.
		Width(o.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1).
		Render(content)

	return dialog
}

func (o *oauthDialogCmp) renderInitialView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Primary).Render("Authenticate with " + o.providerName)
	
	instructions := []string{
		"To use " + o.providerName + ", you'll need to authenticate via OAuth.",
		"",
		"1. Click the button below to open the authentication page in your browser",
		"2. Sign in to your account",
		"3. Authorize Crush to access your account",
		"4. You'll be redirected back to Crush",
		"",
	}
	
	instructionsView := t.S().Base.Foreground(t.FgMuted).Render(
		lipgloss.JoinVertical(lipgloss.Left, instructions...),
	)
	
	button := t.S().Base.
		Background(t.Primary).
		Foreground(t.FgBase).
		Padding(0, 2).
		MarginTop(1).
		Render("Authenticate with " + o.providerName)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		instructionsView,
		button,
	)
}

func (o *oauthDialogCmp) renderRedirectingView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Primary).Render("Redirecting to " + o.providerName + "...")
	message := t.S().Base.Foreground(t.FgMuted).Render("Opening browser for authentication...")
	spinnerView := o.spinner.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
	)
}

func (o *oauthDialogCmp) renderWaitingView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Primary).Render("Waiting for Authorization")
	message := t.S().Base.Foreground(t.FgMuted).Render("Please complete the authentication in your browser...")
	spinnerView := o.spinner.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
	)
}

func (o *oauthDialogCmp) renderExchangingView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Primary).Render("Exchanging Tokens")
	message := t.S().Base.Foreground(t.FgMuted).Render("Exchanging authorization code for access token...")
	spinnerView := o.spinner.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
	)
}

func (o *oauthDialogCmp) renderSuccessView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Green).Render("Authentication Successful!")
	message := t.S().Base.Foreground(t.FgMuted).Render("You're now authenticated with " + o.providerName + ".")

	successDetails := []string{
		"",
		"Your access token has been securely stored.",
		"You can now use " + o.providerName + " with Crush.",
		"",
		"Press Enter to continue...",
	}

	detailsView := t.S().Base.Foreground(t.FgMuted).Render(
		lipgloss.JoinVertical(lipgloss.Left, successDetails...),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		detailsView,
	)
}

func (o *oauthDialogCmp) renderErrorView() string {
	t := styles.CurrentTheme()

	title := t.S().Base.Foreground(t.Cherry).Render("Authentication Failed")
	message := t.S().Base.Foreground(t.FgMuted).Render("Failed to authenticate with " + o.providerName + ".")
	errorMsg := t.S().Base.Foreground(t.Cherry).Render(o.errorMessage)

	tryAgainButton := t.S().Base.
		Background(t.Cherry).
		Foreground(t.FgBase).
		Padding(0, 2).
		MarginTop(1).
		Render("Try Again")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		errorMsg,
		"",
		tryAgainButton,
	)
}

func (o *oauthDialogCmp) Cursor() *tea.Cursor {
	return nil
}

func (o *oauthDialogCmp) Position() (int, int) {
	row := o.wHeight/2 - 5
	col := o.wWidth/2 - o.width/2
	return row, col
}

func (o *oauthDialogCmp) ID() dialogs.DialogID {
	return OAuthDialogID
}

func (o *oauthDialogCmp) StartOAuthFlow() tea.Cmd {
	// This would initiate the OAuth flow
	return func() tea.Msg {
		// Simulate OAuth flow for now
		time.Sleep(2 * time.Second)
		return OAuthStateChangeMsg{
			State: OAuthStateWaiting,
		}
	}
}

func (o *oauthDialogCmp) ExchangeCodeForToken(code string) tea.Cmd {
	// This would exchange the authorization code for an access token
	return func() tea.Msg {
		// Simulate token exchange for now
		time.Sleep(2 * time.Second)
		return OAuthStateChangeMsg{
			State: OAuthStateSuccess,
		}
	}
}

// Close implements the CloseCallback interface
func (o *oauthDialogCmp) Close() tea.Cmd {
	// Reset state when dialog is closed
	o.state = OAuthStateInitial
	o.errorMessage = ""
	o.successMessage = ""
	o.authCode = ""
	return nil
}

// OAuthDialog Interface implementation verification
var _ OAuthDialog = (*oauthDialogCmp)(nil)
var _ dialogs.CloseCallback = (*oauthDialogCmp)(nil)