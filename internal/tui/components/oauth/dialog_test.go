package oauth

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthDialogCmp_Init(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	cmd := dialog.Init()
	assert.NotNil(t, cmd)
}

func TestOAuthDialogCmp_Update_WindowSizeMsg(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	_, cmd := dialog.Update(msg)
	assert.Nil(t, cmd)
	// Verify the width and height were stored
	// Note: The oauthDialogCmp struct stores these as wWidth and wHeight
	// We can't directly access them, but we can test the Position method
}

func TestOAuthDialogCmp_Update_SpinnerTickMsg(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	
	// Test when in a waiting state
	dialog.(*oauthDialogCmp).state = OAuthStateWaiting
	msg := spinner.TickMsg{}
	_, cmd := dialog.Update(msg)
	assert.NotNil(t, cmd)
	
	// Test when not in a waiting state
	dialog.(*oauthDialogCmp).state = OAuthStateInitial
	_, cmd = dialog.Update(msg)
	assert.Nil(t, cmd)
}

func TestOAuthDialogCmp_Update_OAuthStateChangeMsg(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	
	// Test state change without error
	msg := OAuthStateChangeMsg{
		State: OAuthStateSuccess,
		Error: nil,
	}
	_, cmd := dialog.Update(msg)
	assert.Nil(t, cmd)
	assert.Equal(t, OAuthStateSuccess, dialog.(*oauthDialogCmp).state)
	assert.Empty(t, dialog.(*oauthDialogCmp).errorMessage)
	
	// Test state change with error
	msg = OAuthStateChangeMsg{
		State: OAuthStateError,
		Error: &testError{"test error"},
	}
	_, cmd = dialog.Update(msg)
	assert.Nil(t, cmd)
	assert.Equal(t, OAuthStateError, dialog.(*oauthDialogCmp).state)
	assert.Equal(t, "test error", dialog.(*oauthDialogCmp).errorMessage)
}

func TestOAuthDialogCmp_View(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	view := dialog.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authenticate with Test Provider")
}

func TestOAuthDialogCmp_Position(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	dialog.(*oauthDialogCmp).wWidth = 100
	dialog.(*oauthDialogCmp).wHeight = 50
	dialog.(*oauthDialogCmp).width = 60
	
	row, col := dialog.Position()
	// Should be centered: (50/2 - 5, 100/2 - 60/2) = (20, 20)
	assert.Equal(t, 20, row)
	assert.Equal(t, 20, col)
}

func TestOAuthDialogCmp_ID(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	assert.Equal(t, OAuthDialogID, dialog.ID())
}

func TestOAuthDialogCmp_StartOAuthFlow(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	oauthDialog, ok := dialog.(*oauthDialogCmp)
	require.True(t, ok)
	
	cmd := oauthDialog.StartOAuthFlow()
	assert.NotNil(t, cmd)
	
	// Execute the command and verify the result
	result := cmd()
	stateChangeMsg, ok := result.(OAuthStateChangeMsg)
	assert.True(t, ok)
	assert.Equal(t, OAuthStateWaiting, stateChangeMsg.State)
}

func TestOAuthDialogCmp_ExchangeCodeForToken(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	oauthDialog, ok := dialog.(*oauthDialogCmp)
	require.True(t, ok)
	
	cmd := oauthDialog.ExchangeCodeForToken("test-code")
	assert.NotNil(t, cmd)
	
	// Execute the command and verify the result
	result := cmd()
	stateChangeMsg, ok := result.(OAuthStateChangeMsg)
	assert.True(t, ok)
	assert.Equal(t, OAuthStateSuccess, stateChangeMsg.State)
}

func TestOAuthDialogCmp_Close(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	oauthDialog := dialog.(*oauthDialogCmp)
	
	// Set some state values
	oauthDialog.state = OAuthStateError
	oauthDialog.errorMessage = "test error"
	oauthDialog.successMessage = "test success"
	oauthDialog.authCode = "test-code"
	
	// Call Close
	cmd := dialog.Close()
	assert.Nil(t, cmd)
	
	// Verify state was reset
	assert.Equal(t, OAuthStateInitial, oauthDialog.state)
	assert.Empty(t, oauthDialog.errorMessage)
	assert.Empty(t, oauthDialog.successMessage)
	assert.Empty(t, oauthDialog.authCode)
}

// Helper error type for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// Test that the OAuthDialog interface is properly implemented
func TestOAuthDialogInterface(t *testing.T) {
	var _ OAuthDialog = (*oauthDialogCmp)(nil)
	var _ dialogs.DialogModel = (*oauthDialogCmp)(nil)
	var _ dialogs.CloseCallback = (*oauthDialogCmp)(nil)
}