package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQwen3OAuthComponent_ViewStates(t *testing.T) {
	component := NewQwen3OAuthComponent()
	
	// Test initial state view
	component.state = Qwen3OAuthStateInitial
	view := component.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authenticate with Qwen3 Coder")
	assert.Contains(t, view, "you'll need to authenticate via OAuth")
	
	// Test redirecting state view
	component.state = Qwen3OAuthStateRedirecting
	view = component.View()
	assert.Contains(t, view, "Redirecting to Qwen3 Coder")
	assert.Contains(t, view, "Opening browser for authentication")
	
	// Test waiting state view (need to set deviceAuthResp to avoid panic)
	component.state = Qwen3OAuthStateWaiting
	component.deviceAuthResp = &DeviceAuthResponse{
		VerificationURI: "https://example.com",
		UserCode:        "TEST-CODE",
	}
	view = component.View()
	assert.Contains(t, view, "Waiting for Authorization")
	assert.Contains(t, view, "Please complete the authentication in your browser")
	
	// Test exchanging state view
	component.state = Qwen3OAuthStateExchanging
	view = component.View()
	assert.Contains(t, view, "Exchanging Tokens")
	assert.Contains(t, view, "Exchanging authorization code for access token")
	
	// Test success state view
	component.state = Qwen3OAuthStateSuccess
	view = component.View()
	assert.Contains(t, view, "Authentication Successful!")
	assert.Contains(t, view, "You're now authenticated with Qwen3 Coder")
	
	// Test error state view
	component.state = Qwen3OAuthStateError
	component.errorMessage = "Test error message"
	view = component.View()
	assert.Contains(t, view, "Authentication Failed")
	assert.Contains(t, view, "Test error message")
}

func TestQwen3OAuthComponent_renderInitialView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	view := component.renderInitialView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authenticate with Qwen3 Coder")
	assert.Contains(t, view, "1. Click the button below to open the authentication page in your browser")
	assert.Contains(t, view, "Authenticate with Qwen3 Coder")
}

func TestQwen3OAuthComponent_renderRedirectingView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	view := component.renderRedirectingView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Redirecting to Qwen3 Coder")
	assert.Contains(t, view, "Opening browser for authentication")
}

func TestQwen3OAuthComponent_renderWaitingView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	component.deviceAuthResp = &DeviceAuthResponse{
		VerificationURI: "https://example.com",
		UserCode:        "TEST-CODE",
	}
	view := component.renderWaitingView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Waiting for Authorization")
	assert.Contains(t, view, "Please complete the authentication in your browser")
}

func TestQwen3OAuthComponent_renderExchangingView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	view := component.renderExchangingView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Exchanging Tokens")
	assert.Contains(t, view, "Exchanging authorization code for access token")
}

func TestQwen3OAuthComponent_renderSuccessView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	view := component.renderSuccessView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authentication Successful!")
	assert.Contains(t, view, "You're now authenticated with Qwen3 Coder")
}

func TestQwen3OAuthComponent_renderErrorView(t *testing.T) {
	component := NewQwen3OAuthComponent()
	component.errorMessage = "Test error occurred"
	view := component.renderErrorView()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authentication Failed")
	assert.Contains(t, view, "Test error occurred")
	assert.Contains(t, view, "Try Again")
}

func TestQwen3OAuthComponent_Cursor(t *testing.T) {
	component := NewQwen3OAuthComponent()
	
	// Test cursor in waiting state (need to set deviceAuthResp and size)
	component.state = Qwen3OAuthStateWaiting
	component.width = 80
	component.height = 24
	component.deviceAuthResp = &DeviceAuthResponse{
		VerificationURI: "https://example.com",
		UserCode:        "TEST-CODE",
	}
	cursor := component.Cursor()
	// In waiting state, cursor behavior depends on the input component
	// We're not testing the specific behavior here, just that it doesn't panic
	
	// Test cursor in non-waiting state
	component.state = Qwen3OAuthStateInitial
	cursor = component.Cursor()
	assert.Nil(t, cursor)
}

func TestOAuthDialogCmp_ViewStates(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	oauthDialog := dialog.(*oauthDialogCmp)
	
	// Test initial state view
	oauthDialog.state = OAuthStateInitial
	view := oauthDialog.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Authenticate with Test Provider")
	
	// Test redirecting state view
	oauthDialog.state = OAuthStateRedirecting
	view = oauthDialog.View()
	assert.Contains(t, view, "Redirecting to Test Provider")
	
	// Test waiting state view
	oauthDialog.state = OAuthStateWaiting
	view = oauthDialog.View()
	assert.Contains(t, view, "Waiting for Authorization")
	
	// Test exchanging state view
	oauthDialog.state = OAuthStateExchanging
	view = oauthDialog.View()
	assert.Contains(t, view, "Exchanging Tokens")
	
	// Test success state view
	oauthDialog.state = OAuthStateSuccess
	view = oauthDialog.View()
	assert.Contains(t, view, "Authentication Successful!")
	
	// Test error state view
	oauthDialog.state = OAuthStateError
	oauthDialog.errorMessage = "Test error"
	view = oauthDialog.View()
	assert.Contains(t, view, "Authentication Failed")
	assert.Contains(t, view, "Test error")
}

func TestQwen3OAuthComponent_MessagePassing(t *testing.T) {
	component := NewQwen3OAuthComponent()
	
	// Test Qwen3OAuthStateChangeMsg
	msg := Qwen3OAuthStateChangeMsg{
		State: Qwen3OAuthStateSuccess,
		Error: nil,
	}
	
	_, cmd := component.Update(msg)
	// For success state, should return a command to close the dialog
	assert.NotNil(t, cmd)
	assert.Equal(t, Qwen3OAuthStateSuccess, component.state)
	
	// Test with error
	errorMsg := Qwen3OAuthStateChangeMsg{
		State: Qwen3OAuthStateError,
		Error: &testError{"test error"},
	}
	
	_, cmd = component.Update(errorMsg)
	// For error state, should not return a command to close the dialog immediately
	assert.Nil(t, cmd)
	assert.Equal(t, Qwen3OAuthStateError, component.state)
	assert.Equal(t, "test error", component.errorMessage)
}

// testError is defined in dialog_test.go

func TestOAuthDialogCmp_MessagePassing(t *testing.T) {
	dialog := NewOAuthDialogCmp("Test Provider")
	oauthDialog := dialog.(*oauthDialogCmp)
	
	// Test OAuthStateChangeMsg
	msg := OAuthStateChangeMsg{
		State: OAuthStateSuccess,
		Error: nil,
	}
	
	_, cmd := oauthDialog.Update(msg)
	assert.Nil(t, cmd)
	assert.Equal(t, OAuthStateSuccess, oauthDialog.state)
	
	// Test with error
	errorMsg := OAuthStateChangeMsg{
		State: OAuthStateError,
		Error: &testError{"test error"},
	}
	
	_, cmd = oauthDialog.Update(errorMsg)
	assert.Nil(t, cmd)
	assert.Equal(t, OAuthStateError, oauthDialog.state)
	assert.Equal(t, "test error", oauthDialog.errorMessage)
}