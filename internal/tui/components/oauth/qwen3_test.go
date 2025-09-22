package oauth

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQwen3OAuthComponent_Init(t *testing.T) {
	component := NewQwen3OAuthComponent()
	cmd := component.Init()
	assert.NotNil(t, cmd)
}

func TestQwen3OAuthComponent_Update_WindowSizeMsg(t *testing.T) {
	component := NewQwen3OAuthComponent()
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, cmd := component.Update(msg)
	assert.Nil(t, cmd)
	assert.Equal(t, 80, component.width)
	assert.Equal(t, 24, component.height)
}

func TestQwen3OAuthComponent_Update_EscapeKey(t *testing.T) {
	component := NewQwen3OAuthComponent()
	msg := tea.KeyPressMsg{
		Code: tea.KeyEscape,
	}
	_, cmd := component.Update(msg)
	assert.NotNil(t, cmd)
	assert.Equal(t, Qwen3OAuthStateError, component.state)
	assert.Equal(t, "Authentication cancelled by user", component.errorMessage)
}

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := generateCodeVerifier()
	require.NoError(t, err)
	assert.NotEmpty(t, verifier)
	assert.Greater(t, len(verifier), 32) // Should be at least 32 bytes base64 encoded
}

func TestGenerateCodeChallenge(t *testing.T) {
	testCases := []struct {
		verifier  string
		expected  string
		name      string
	}{
		{
			verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
			expected: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			name:     "RFC 7636 example",
		},
		{
			verifier: "test-verifier-for-testing-purposes-with-sufficient-length",
			name:     "Custom test verifier",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			challenge := generateCodeChallenge(tc.verifier)
			assert.NotEmpty(t, challenge)
			
			// Challenge should be 43 characters long (base64url encoded SHA256)
			assert.Equal(t, 43, len(challenge))
		})
	}
}

func TestQwen3OAuthComponent_Position(t *testing.T) {
	component := NewQwen3OAuthComponent()
	component.width = 80
	component.height = 24
	
	row, col := component.Position()
	// Should be centered: (24/2 - 5, 80/2 - 80/2) = (7, 0)
	assert.Equal(t, 7, row)
	assert.Equal(t, 0, col)
}

func TestQwen3OAuthComponent_ID(t *testing.T) {
	component := NewQwen3OAuthComponent()
	assert.Equal(t, "qwen3-oauth", string(component.ID()))
}

func TestQwen3OAuthComponent_Close(t *testing.T) {
	component := NewQwen3OAuthComponent()
	component.state = Qwen3OAuthStateSuccess
	component.errorMessage = "test error"
	component.successMessage = "test success"
	component.deviceAuthResp = &DeviceAuthResponse{}
	component.codeVerifier = "test-verifier"
	component.pollInterval = 10 * time.Second
	component.maxAttempts = 50
	component.attemptCount = 10
	
	cmd := component.Close()
	assert.Nil(t, cmd)
	assert.Equal(t, Qwen3OAuthStateInitial, component.state)
	assert.Empty(t, component.errorMessage)
	assert.Empty(t, component.successMessage)
	assert.Nil(t, component.deviceAuthResp)
	assert.Empty(t, component.codeVerifier)
	assert.Equal(t, 2*time.Second, component.pollInterval)
	assert.Equal(t, 30, component.maxAttempts)
	assert.Equal(t, 0, component.attemptCount)
}

func TestQwen3OAuthComponent_SetSize(t *testing.T) {
	component := NewQwen3OAuthComponent()
	component.SetSize(100, 50)
	assert.Equal(t, 100, component.width)
	assert.Equal(t, 50, component.height)
	// Input width should be adjusted (width - 4)
	assert.Equal(t, 96, component.input.Width())
}