package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/pkg/browser"
)

// Qwen OAuth Configuration
const (
	QwenOAuthBaseURL        = "https://chat.qwen.ai"
	QwenDeviceCodeEndpoint  = QwenOAuthBaseURL + "/api/v1/oauth2/device/code"
	QwenTokenEndpoint       = QwenOAuthBaseURL + "/api/v1/oauth2/token"
	QwenModelsEndpoint      = QwenOAuthBaseURL + "/api/v1/models"
	QwenDefaultClientID     = "f0304373b74a44d2b584a3fb70ca9e56" // OAuth 2.0 public client ID (safe to embed per RFC 8628)
	QwenScope               = "openid profile email model.completion"
	QwenGrantType           = "urn:ietf:params:oauth:grant-type:device_code"
	QwenRefreshGrantType    = "refresh_token"
	QwenCodeChallengeMethod = "S256"
)

// getQwenClientID returns the Qwen OAuth client ID, checking environment variable first.
// For OAuth 2.0 Device Flow (RFC 8628), client IDs are public and safe to embed.
// Environment variable allows override for testing or alternative configurations.
func getQwenClientID() string {
	if clientID := os.Getenv("QWEN_CLIENT_ID"); clientID != "" {
		slog.Debug("Using Qwen client ID from environment variable")
		return clientID
	}
	return QwenDefaultClientID
}

// qwenModelSpecs maps known Qwen model IDs to their actual specifications.
// Based on official Qwen documentation and research (2025).
var qwenModelSpecs = map[string]catwalk.Model{
	// Qwen3-Coder models (coding-specialized, NO vision support)
	"qwen3-coder-480b": {
		ContextWindow:      262144, // 256K native, up to 1M with extrapolation
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false, // Qwen3-Coder does NOT support images
	},
	"qwen3-coder": {
		ContextWindow:      262144,
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	// Qwen3 general models
	"qwen3-32b": {
		ContextWindow:          131072, // 128K
		DefaultMaxTokens:       8192,
		CanReason:              true,
		HasReasoningEffort:     true, // Only Qwen3-32B supports reasoning_effort
		DefaultReasoningEffort: "medium",
		SupportsImages:         false,
	},
	"qwen3-235b": {
		ContextWindow:      131072,
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	// Qwen Max/Plus/Turbo API models
	"qwen-max": {
		ContextWindow:      262144, // 256K tokens
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	"qwen-plus": {
		ContextWindow:      1000000, // 1M tokens
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	"qwen-turbo": {
		ContextWindow:      131072, // 128K tokens
		DefaultMaxTokens:   4096,
		CanReason:          false,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	"qwen-flash": {
		ContextWindow:      131072, // 128K tokens
		DefaultMaxTokens:   4096,
		CanReason:          false,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	// Qwen 2.5 models with long context
	"qwen2.5-72b": {
		ContextWindow:      131072, // 128K base
		DefaultMaxTokens:   8192,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	"qwen2.5-7b": {
		ContextWindow:      131072,
		DefaultMaxTokens:   4096,
		CanReason:          false,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	"qwen2.5-14b": {
		ContextWindow:      131072,
		DefaultMaxTokens:   4096,
		CanReason:          true,
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
	// QwQ reasoning models
	"qwq-32b": {
		ContextWindow:      32768,
		DefaultMaxTokens:   4096,
		CanReason:          true, // Dedicated reasoning model
		HasReasoningEffort: false,
		SupportsImages:     false,
	},
}

// getModelSpecs returns the model specifications for a given model ID.
// If the exact model ID is not found, it attempts pattern matching.
// Falls back to conservative defaults if no match is found.
func getModelSpecs(modelID string) catwalk.Model {
	// Direct match
	if spec, exists := qwenModelSpecs[modelID]; exists {
		slog.Debug("Found exact model specs", "model", modelID, "context_window", spec.ContextWindow)
		return spec
	}

	// Pattern matching for model variants (e.g., "qwen3-coder-30b" matches "qwen3-coder")
	lowerID := strings.ToLower(modelID)
	for knownID, spec := range qwenModelSpecs {
		if strings.Contains(lowerID, knownID) {
			slog.Debug("Found model specs via pattern match", "model", modelID, "matched", knownID, "context_window", spec.ContextWindow)
			return spec
		}
	}

	// Conservative defaults for unknown models
	slog.Warn("Unknown Qwen model, using conservative defaults", "model", modelID)
	return catwalk.Model{
		ContextWindow:      32768, // Conservative 32K default
		DefaultMaxTokens:   4096,
		CanReason:          false, // Conservative: assume no reasoning
		HasReasoningEffort: false,
		SupportsImages:     false, // Conservative: assume no vision
	}
}

// getFallbackModels returns a default set of Qwen models when API fetch fails.
// These are common models available via Qwen OAuth.
func getFallbackModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:                 "qwen3-coder",
			Name:               "Qwen3 Coder",
			ContextWindow:      262144,
			DefaultMaxTokens:   8192,
			CanReason:          true,
			HasReasoningEffort: false,
			SupportsImages:     false,
		},
		{
			ID:               "qwen-plus",
			Name:             "Qwen Plus",
			ContextWindow:    1000000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "qwen-turbo",
			Name:             "Qwen Turbo",
			ContextWindow:    131072,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
	}
}

// Device Authorization Response
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval,omitempty"`
	SlowDown                bool   `json:"slow_down,omitempty"`
}

// Token Response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
}

// Models Response
type ModelsResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

type Qwen3OAuthState int

const (
	Qwen3OAuthStateInitial Qwen3OAuthState = iota
	Qwen3OAuthStateRedirecting
	Qwen3OAuthStateWaiting
	Qwen3OAuthStateExchanging
	Qwen3OAuthStateSuccess
	Qwen3OAuthStateError
)

type Qwen3OAuthStateChangeMsg struct {
	State Qwen3OAuthState
	Error error
}

type Qwen3OAuthComponent struct {
	width          int
	height         int
	state          Qwen3OAuthState
	spinner        spinner.Model
	input          textinput.Model
	errorMessage   string
	successMessage string
	providerName   string
	// OAuth flow fields
	deviceAuthResp *DeviceAuthResponse
	codeVerifier   string
	pollInterval   time.Duration
	maxAttempts    int
	attemptCount   int
}

func NewQwen3OAuthComponent() *Qwen3OAuthComponent {
	t := styles.CurrentTheme()
	ti := textinput.New()
	ti.Placeholder = "Paste your authorization code here..."
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.S().Base.Foreground(t.Green)),
	)
	return &Qwen3OAuthComponent{
		state:        Qwen3OAuthStateInitial,
		spinner:      s,
		input:        ti,
		providerName: "Qwen3 Coder",
		pollInterval: 2 * time.Second, // Start with 2 seconds
		maxAttempts:  30,              // Default max attempts
		attemptCount: 0,
	}
}

func (q *Qwen3OAuthComponent) Init() tea.Cmd {
	q.updateStatePresentation()
	// Start the OAuth flow automatically when the dialog opens
	return tea.Batch(
		q.spinner.Tick,
		q.StartOAuthFlow(),
	)
}

func (q *Qwen3OAuthComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if q.state == Qwen3OAuthStateRedirecting || q.state == Qwen3OAuthStateWaiting || q.state == Qwen3OAuthStateExchanging {
			var cmd tea.Cmd
			q.spinner, cmd = q.spinner.Update(msg)
			q.updateStatePresentation()
			return q, cmd
		}
		return q, nil
	case Qwen3OAuthStateChangeMsg:
		q.state = msg.State
		if msg.Error != nil {
			q.errorMessage = msg.Error.Error()
		}
		var cmd tea.Cmd
		if msg.State == Qwen3OAuthStateRedirecting || msg.State == Qwen3OAuthStateWaiting || msg.State == Qwen3OAuthStateExchanging {
			cmd = q.spinner.Tick
		} else if msg.State == Qwen3OAuthStateSuccess {
			// Close the dialog after a short delay
			cmd = tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
				return dialogs.CloseDialogMsg{}
			})
		}
		q.updateStatePresentation()
		return q, cmd
	case tea.WindowSizeMsg:
		q.width = msg.Width
		q.height = msg.Height
		q.input.SetWidth(q.width - 4)
		return q, nil
	case time.Time:
		// Handle polling tick for device code exchange
		if q.state == Qwen3OAuthStateWaiting {
			// Start polling for tokens
			return q, tea.Batch(
				q.spinner.Tick,
				func() tea.Msg {
					return q.startPollingForTokens()
				},
			)
		}
	case tea.KeyPressMsg:
		// Handle Escape key to cancel
		if msg.String() == "esc" {
			q.state = Qwen3OAuthStateError
			q.errorMessage = "Authentication cancelled by user"
			q.updateStatePresentation()
			return q, func() tea.Msg {
				return dialogs.CloseDialogMsg{}
			}
		}
	case dialogs.CloseDialogMsg:
		// Handle dialog close message
		return q, nil
	}
	var cmd tea.Cmd
	q.input, cmd = q.input.Update(msg)
	return q, cmd
}

func (q *Qwen3OAuthComponent) updateStatePresentation() {
	// No special styling needed for state presentation
}

func (q *Qwen3OAuthComponent) View() string {
	t := styles.CurrentTheme()
	var content string
	switch q.state {
	case Qwen3OAuthStateInitial:
		content = q.renderInitialView()
	case Qwen3OAuthStateRedirecting:
		content = q.renderRedirectingView()
	case Qwen3OAuthStateWaiting:
		content = q.renderWaitingView()
	case Qwen3OAuthStateExchanging:
		content = q.renderExchangingView()
	case Qwen3OAuthStateSuccess:
		content = q.renderSuccessView()
	case Qwen3OAuthStateError:
		content = q.renderErrorView()
	}
	return t.S().Base.
		Width(q.width).
		Height(q.height).
		Padding(1).
		Render(content)
}

func (q *Qwen3OAuthComponent) renderInitialView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render("Authenticate with " + q.providerName)
	instructions := []string{
		"To use " + q.providerName + ", you'll need to authenticate via OAuth.",
		"",
		"1. Click the button below to open the authentication page in your browser",
		"2. Sign in to your Qwen account",
		"3. Authorize Crush to access your account",
		"4. You'll be redirected back to Crush with an authorization code",
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
		Render("Authenticate with " + q.providerName)
	dataPath := config.GlobalConfigData()
	dataPath = home.Short(dataPath)
	helpText := t.S().Muted.Render(fmt.Sprintf("Tokens will be stored in: %s", dataPath))
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		instructionsView,
		button,
		"",
		helpText,
	)
}

func (q *Qwen3OAuthComponent) renderRedirectingView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render("Redirecting to " + q.providerName + "...")
	message := t.S().Base.Foreground(t.FgMuted).Render("Opening browser for authentication...")
	spinnerView := q.spinner.View()
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
	)
}

func (q *Qwen3OAuthComponent) renderWaitingView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render("Waiting for Authorization")
	message := t.S().Base.Foreground(t.FgMuted).Render("Please complete the authentication in your browser...")
	spinnerView := q.spinner.View()
	manualInstructions := []string{
		"",
		"If your browser didn't open automatically:",
		"1. Copy this URL: " + q.deviceAuthResp.VerificationURI,
		"2. Paste it in your browser",
		"3. Enter this code when prompted: " + q.deviceAuthResp.UserCode,
		"4. Complete the authentication process",
		"",
		"The application is automatically polling for your authorization.",
		"Please be patient, this may take a few moments...",
		"",
	}
	manualView := t.S().Base.Foreground(t.FgMuted).Render(
		lipgloss.JoinVertical(lipgloss.Left, manualInstructions...),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
		manualView,
	)
}

func (q *Qwen3OAuthComponent) renderExchangingView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render("Exchanging Tokens")
	message := t.S().Base.Foreground(t.FgMuted).Render("Exchanging authorization code for access token...")
	spinnerView := q.spinner.View()
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		spinnerView,
	)
}

func (q *Qwen3OAuthComponent) renderSuccessView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Green).Render("Authentication Successful!")
	message := t.S().Base.Foreground(t.FgMuted).Render("You're now authenticated with " + q.providerName + ".")
	successDetails := []string{
		"",
		"Your access token has been securely stored.",
		"You can now use " + q.providerName + " with Crush.",
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

func (q *Qwen3OAuthComponent) renderErrorView() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Cherry).Render("Authentication Failed")
	message := t.S().Base.Foreground(t.FgMuted).Render("Failed to authenticate with " + q.providerName + ".")
	errorMsg := t.S().Base.Foreground(t.Cherry).Render(q.errorMessage)
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

func (q *Qwen3OAuthComponent) Cursor() *tea.Cursor {
	if q.state == Qwen3OAuthStateWaiting {
		cursor := q.input.Cursor()
		if cursor != nil {
			cursor.Y += 10 // Adjust for title and spacing
		}
		return cursor
	}
	return nil
}

// generateCodeVerifier generates a cryptographically random code verifier
func generateCodeVerifier() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Encode using base64url encoding
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateCodeChallenge generates a PKCE code challenge from the code verifier
func generateCodeChallenge(codeVerifier string) string {
	// Hash the code verifier using SHA256
	hash := sha256.Sum256([]byte(codeVerifier))
	// Encode using base64url encoding
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// startPollingForTokens initiates the polling process to exchange the device code for tokens
func (q *Qwen3OAuthComponent) startPollingForTokens() tea.Msg {
	// Increment the attempt counter
	q.attemptCount++
	slog.Info("OAuth polling attempt", "attempt", q.attemptCount, "max_attempts", q.maxAttempts)

	// Check if we've exceeded the maximum attempts
	if q.attemptCount >= q.maxAttempts {
		slog.Error("OAuth authentication timed out", "attempts", q.attemptCount)
		return Qwen3OAuthStateChangeMsg{
			State: Qwen3OAuthStateError,
			Error: fmt.Errorf("authentication timed out after %d attempts", q.attemptCount),
		}
	}

	// Attempt to exchange the device code for tokens
	tokenResp, err := q.exchangeDeviceCodeForTokens(q.deviceAuthResp.DeviceCode)
	if err != nil {
		slog.Debug("OAuth token exchange failed", "error", err)
		// Check if this is a pending response that requires continued polling
		// Use more robust error checking instead of exact string matching
		errStr := err.Error()
		if strings.Contains(errStr, "authorization_pending") {
			slog.Debug("Authorization pending, continuing to poll")
			// Schedule the next polling attempt
			return tea.Tick(q.pollInterval, func(t time.Time) tea.Msg {
				return q.startPollingForTokens()
			})
		} else if strings.Contains(errStr, "slow_down") {
			slog.Debug("Server requested slow down")
			// Increase the polling interval and schedule next attempt
			q.pollInterval = time.Duration(float64(q.pollInterval) * 1.5) // Increase by 50%
			if q.pollInterval > 10*time.Second {
				q.pollInterval = 10 * time.Second // Cap at 10 seconds
			}
			slog.Info("Adjusted polling interval", "new_interval", q.pollInterval)
			return tea.Tick(q.pollInterval, func(t time.Time) tea.Msg {
				return q.startPollingForTokens()
			})
		} else {
			// Another error occurred, stop polling and show error
			slog.Error("OAuth token exchange failed", "error", err)
			return Qwen3OAuthStateChangeMsg{
				State: Qwen3OAuthStateError,
				Error: fmt.Errorf("failed to exchange device code: %w", err),
			}
		}
	}

	// Successfully obtained tokens
	slog.Info("OAuth token exchange successful")
	// Store the tokens securely
	cfg := config.Get()
	currentTime := time.Now().Unix()

	// Save the provider credentials
	err = cfg.SetProviderCredentials("qwen3-coder-oauth", "", tokenResp.AccessToken, tokenResp.RefreshToken, currentTime+int64(tokenResp.ExpiresIn), "oauth")
	if err != nil {
		slog.Error("Failed to save OAuth tokens", "error", err)
		return Qwen3OAuthStateChangeMsg{
			State: Qwen3OAuthStateError,
			Error: fmt.Errorf("failed to save OAuth tokens: %w", err),
		}
	}

	// Fetch available models from Qwen API
	slog.Info("Fetching models from Qwen API")
	models, fetchErr := q.fetchModels(tokenResp.AccessToken)

	var catwalkModels []catwalk.Model

	if fetchErr != nil {
		slog.Error("Failed to fetch models from Qwen API", "error", fetchErr)
		slog.Warn("Using fallback models due to API fetch failure")
		// Use fallback models when API fetch fails
		catwalkModels = getFallbackModels()
	} else {
		slog.Info("Successfully fetched models from Qwen API", "model_count", len(models.Data))
		// Convert the fetched models to catwalk models with proper specifications
		catwalkModels = make([]catwalk.Model, len(models.Data))
		for i, model := range models.Data {
			// Get the proper model specifications based on the model ID
			specs := getModelSpecs(model.ID)

			// Copy specs and set ID and Name from the API response
			catwalkModels[i] = specs
			catwalkModels[i].ID = model.ID
			catwalkModels[i].Name = model.ID
		}
	}

	// Update the provider config with the models (either fetched or fallback)
	cfg := config.Get()
	providerConfig, exists := cfg.Providers.Get("qwen3-coder-oauth")
	if !exists {
		slog.Error("Provider config not found for qwen3-coder-oauth")
		// Continue anyway with success state - authentication worked
		return tea.Sequence(
			func() tea.Msg {
				return Qwen3OAuthStateChangeMsg{
					State: Qwen3OAuthStateSuccess,
				}
			},
			tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
				slog.Info("Closing OAuth dialog")
				return dialogs.CloseDialogMsg{}
			}),
		)
	}

	providerConfig.Models = catwalkModels
	cfg.Providers.Set("qwen3-coder-oauth", providerConfig)

	// Send a message to update the model list in the UI
	return tea.Sequence(
		func() tea.Msg {
			return message.ModelsUpdateMsg{
				ProviderID: "qwen3-coder-oauth",
				Models:     catwalkModels,
			}
		},
		func() tea.Msg {
			return Qwen3OAuthStateChangeMsg{
				State: Qwen3OAuthStateSuccess,
			}
		},
		tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			slog.Info("Closing OAuth dialog")
			return dialogs.CloseDialogMsg{}
		}),
	)
}

// fetchModels retrieves the list of available models from the Qwen API
func (q *Qwen3OAuthComponent) fetchModels(accessToken string) (*ModelsResponse, error) {
	slog.Debug("Fetching models from Qwen API")

	// Create the HTTP request
	req, err := http.NewRequest("GET", QwenModelsEndpoint, nil)
	if err != nil {
		slog.Error("Failed to create models request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to fetch models", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("Models response received", "status_code", resp.StatusCode)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		// Create a copy of the response body for logging
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Error("Failed to read response body for error parsing", "error", readErr)
		} else {
			slog.Debug("Response body for models error", "body", string(bodyBytes))
			// Parse the error response from the copied body
			if decodeErr := json.Unmarshal(bodyBytes, &errorResp); decodeErr == nil {
				if errorResp.Error != "" {
					if errorResp.ErrorDescription != "" {
						slog.Error("Models fetch error", "error", errorResp.Error, "description", errorResp.ErrorDescription)
						return nil, fmt.Errorf("%s: %s", errorResp.Error, errorResp.ErrorDescription)
					}
					slog.Error("Models fetch error", "error", errorResp.Error)
					return nil, fmt.Errorf("%s", errorResp.Error)
				}
			}
		}
		slog.Error("Models fetch failed", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("models fetch request failed with status %d", resp.StatusCode)
	}

	// Parse the successful response
	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		slog.Error("Failed to parse models response", "error", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	slog.Debug("Models response parsed", "model_count", len(modelsResp.Data))

	return &modelsResp, nil
}
func (q *Qwen3OAuthComponent) exchangeDeviceCodeForTokens(deviceCode string) (*TokenResponse, error) {
	slog.Debug("Attempting to exchange device code for tokens")

	// Log request details for debugging
	clientID := getQwenClientID()
	slog.Debug("Token exchange request details",
		"grant_type", QwenGrantType,
		"client_id", clientID,
		"device_code_length", len(deviceCode),
		"code_verifier_length", len(q.codeVerifier))

	// Prepare the request body
	data := url.Values{}
	data.Set("grant_type", QwenGrantType)
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)
	data.Set("code_verifier", q.codeVerifier)
	// Create the HTTP request
	req, err := http.NewRequest("POST", QwenTokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		slog.Error("Failed to create token exchange request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", uuid.New().String())
	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to make token exchange request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("Token exchange response received",
		"status_code", resp.StatusCode,
		"headers", resp.Header,
		"content_length", resp.ContentLength)
	// Check response status
	if resp.StatusCode == http.StatusBadRequest {
		// Parse error response to determine if we should continue polling
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errorResp); decodeErr == nil {
			slog.Debug("Parsed error response", "error", errorResp.Error, "description", errorResp.ErrorDescription)
			if errorResp.Error == "authorization_pending" {
				// This is expected - user hasn't completed auth yet, continue polling
				slog.Debug("Authorization pending, will continue polling")
				return nil, fmt.Errorf("authorization_pending")
			} else if errorResp.Error == "slow_down" {
				// Server is asking us to slow down our polling
				slog.Debug("Server requested slow down")
				return nil, fmt.Errorf("slow_down")
			} else if errorResp.Error == "expired_token" {
				// The device code has expired
				slog.Error("Device code expired", "description", errorResp.ErrorDescription)
				return nil, fmt.Errorf("expired_token: %s", errorResp.ErrorDescription)
			} else if errorResp.Error == "access_denied" {
				// User denied the authorization request
				slog.Error("Access denied by user", "description", errorResp.ErrorDescription)
				return nil, fmt.Errorf("access_denied: %s", errorResp.ErrorDescription)
			} else if errorResp.Error != "" {
				// Some other error occurred
				if errorResp.ErrorDescription != "" {
					slog.Error("Token exchange error", "error", errorResp.Error, "description", errorResp.ErrorDescription)
					return nil, fmt.Errorf("%s: %s", errorResp.Error, errorResp.ErrorDescription)
				}
				slog.Error("Token exchange error", "error", errorResp.Error)
				return nil, fmt.Errorf("%s", errorResp.Error)
			}
		} else {
			slog.Error("Failed to parse error response", "error", decodeErr)
		}
		slog.Error("Token exchange failed with 400 status", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("token exchange request failed with status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		// Try to parse error response even for non-200 status codes
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		// Create a copy of the response body for logging
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Error("Failed to read response body for error parsing", "error", readErr)
		} else {
			slog.Debug("Response body for non-200 status", "body", string(bodyBytes))
			// Parse the error response from the copied body
			if decodeErr := json.Unmarshal(bodyBytes, &errorResp); decodeErr == nil && errorResp.Error != "" {
				if errorResp.ErrorDescription != "" {
					slog.Error("Token exchange error", "error", errorResp.Error, "description", errorResp.ErrorDescription)
					return nil, fmt.Errorf("%s: %s", errorResp.Error, errorResp.ErrorDescription)
				}
				slog.Error("Token exchange error", "error", errorResp.Error)
				return nil, fmt.Errorf("%s", errorResp.Error)
			}
		}
		slog.Error("Token exchange failed", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("token exchange request failed with status %d", resp.StatusCode)
	}
	// Parse the successful response
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		slog.Error("Failed to parse token response", "error", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	slog.Debug("Token response parsed",
		"access_token_present", tokenResp.AccessToken != "",
		"refresh_token_present", tokenResp.RefreshToken != "",
		"token_type", tokenResp.TokenType,
		"expires_in", tokenResp.ExpiresIn,
		"scope", tokenResp.Scope)

	// Check if there was an error in the response
	if tokenResp.Error != "" {
		slog.Error("Token response contains error", "error", tokenResp.Error)
		return nil, fmt.Errorf("token exchange error: %s", tokenResp.Error)
	}

	// Verify we got the required tokens and information
	if tokenResp.AccessToken == "" {
		slog.Error("Token exchange succeeded but no access token was returned")
		return nil, fmt.Errorf("token exchange succeeded but no access token was returned")
	}

	// Set default token type if not provided
	if tokenResp.TokenType == "" {
		slog.Warn("Token response missing token_type, defaulting to Bearer")
		tokenResp.TokenType = "Bearer"
	}

	// Log successful token exchange with all relevant information
	slog.Info("Token exchange successful",
		"access_token_length", len(tokenResp.AccessToken),
		"token_type", tokenResp.TokenType,
		"refresh_token_present", tokenResp.RefreshToken != "",
		"expires_in", tokenResp.ExpiresIn,
		"scope", tokenResp.Scope)

	return &tokenResp, nil
}

func (q *Qwen3OAuthComponent) SetSize(width, height int) {
	q.width = width
	q.height = height
	q.input.SetWidth(width - 4)
}

func (q *Qwen3OAuthComponent) Position() (int, int) {
	row := q.height/2 - 5
	col := q.width/2 - q.width/2
	return row, col
}

func (q *Qwen3OAuthComponent) StartOAuthFlow() tea.Cmd {
	// This initiates the OAuth flow by requesting device authorization
	slog.Info("Starting OAuth flow")

	return func() tea.Msg {
		slog.Debug("Generating PKCE code verifier and challenge")
		// Generate PKCE code verifier and challenge
		codeVerifier, err := generateCodeVerifier()
		if err != nil {
			slog.Error("Failed to generate code verifier", "error", err)
			return Qwen3OAuthStateChangeMsg{
				State: Qwen3OAuthStateError,
				Error: fmt.Errorf("failed to generate code verifier: %w", err),
			}
		}

		slog.Debug("Generated code verifier", "length", len(codeVerifier))
		codeChallenge := generateCodeChallenge(codeVerifier)
		slog.Debug("Generated code challenge", "length", len(codeChallenge))

		// Request device authorization from Qwen OAuth server
		slog.Info("Requesting device authorization from Qwen OAuth server")
		deviceAuthResp, err := q.requestDeviceAuthorization(codeVerifier, codeChallenge)
		if err != nil {
			slog.Error("Failed to request device authorization", "error", err)
			return Qwen3OAuthStateChangeMsg{
				State: Qwen3OAuthStateError,
				Error: fmt.Errorf("failed to request device authorization: %w", err),
			}
		}

		slog.Info("Device authorization successful",
			"user_code", deviceAuthResp.UserCode,
			"verification_uri", deviceAuthResp.VerificationURI)

		// Store the device auth response and code verifier for later use
		q.deviceAuthResp = deviceAuthResp
		q.codeVerifier = codeVerifier

		// Set polling interval and max attempts
		if deviceAuthResp.Interval > 0 {
			q.pollInterval = time.Duration(deviceAuthResp.Interval) * time.Second
		} else {
			q.pollInterval = 5 * time.Second // Default to 5 seconds if not provided
		}

		// Calculate max attempts based on expiration time (with 20% buffer)
		// Typical device codes expire in 1800 seconds (30 minutes)
		q.maxAttempts = int(float64(deviceAuthResp.ExpiresIn) / q.pollInterval.Seconds() * 1.2)
		if q.maxAttempts < 10 {
			q.maxAttempts = 10 // Minimum attempts (for very short expirations)
		}
		if q.maxAttempts > 360 {
			q.maxAttempts = 360 // Maximum attempts (30 min at 5s intervals)
		}
		q.attemptCount = 0

		slog.Debug("OAuth polling configuration",
			"interval", q.pollInterval,
			"max_attempts", q.maxAttempts,
			"expires_in", deviceAuthResp.ExpiresIn,
			"estimated_timeout_seconds", int(float64(q.maxAttempts)*q.pollInterval.Seconds()))

		// Try to open the verification URL in the browser
		var openErr error
		if deviceAuthResp.VerificationURIComplete != "" {
			slog.Info("Opening browser with complete verification URL", "url", deviceAuthResp.VerificationURIComplete)
			openErr = browser.OpenURL(deviceAuthResp.VerificationURIComplete)
		} else {
			slog.Info("Opening browser with verification URL", "url", deviceAuthResp.VerificationURI)
			openErr = browser.OpenURL(deviceAuthResp.VerificationURI)
		}

		// Update state to waiting
		stateMsg := Qwen3OAuthStateChangeMsg{
			State: Qwen3OAuthStateWaiting,
		}

		// If browser opening fails, we'll just show the URL for manual navigation
		// The user can copy and paste it manually
		if openErr != nil {
			slog.Warn("Failed to open browser for OAuth", "error", openErr)
			// Log the error but don't treat it as fatal
			// The user can still manually navigate to the URL
		}

		// Start polling for token exchange after a short delay
		slog.Info("Starting OAuth polling", "interval", q.pollInterval, "max_attempts", q.maxAttempts)
		return tea.Sequence(
			func() tea.Msg { return stateMsg },
			tea.Tick(q.pollInterval, func(t time.Time) tea.Msg {
				return q.startPollingForTokens()
			}),
		)
	}
}

// requestDeviceAuthorization requests device authorization from Qwen OAuth server
func (q *Qwen3OAuthComponent) requestDeviceAuthorization(codeVerifier string, codeChallenge string) (*DeviceAuthResponse, error) {
	slog.Debug("Requesting device authorization")

	// Validate required parameters
	if codeVerifier == "" || codeChallenge == "" {
		slog.Error("Missing required parameters for device authorization")
		return nil, fmt.Errorf("code verifier and challenge are required")
	}

	// Prepare the request body
	clientID := getQwenClientID()
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", QwenScope)
	data.Set("code_challenge", codeChallenge)
	data.Set("code_challenge_method", QwenCodeChallengeMethod)

	slog.Debug("Device authorization request prepared",
		"client_id", clientID,
		"scope", QwenScope,
		"code_challenge_method", QwenCodeChallengeMethod)

	// Create the HTTP request
	req, err := http.NewRequest("POST", QwenDeviceCodeEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		slog.Error("Failed to create device authorization request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-request-id", uuid.New().String())

	slog.Debug("Device authorization request created",
		"url", QwenDeviceCodeEndpoint,
		"method", "POST")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to make device authorization request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("Device authorization response received",
		"status_code", resp.StatusCode,
		"headers", resp.Header)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		// Create a copy of the response body for logging
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Error("Failed to read response body for error parsing", "error", readErr)
		} else {
			slog.Debug("Response body for device auth error", "body", string(bodyBytes))
			// Parse the error response from the copied body
			if decodeErr := json.Unmarshal(bodyBytes, &errorResp); decodeErr == nil {
				if errorResp.Error != "" {
					if errorResp.ErrorDescription != "" {
						slog.Error("Device authorization error", "error", errorResp.Error, "description", errorResp.ErrorDescription)
						return nil, fmt.Errorf("device authorization error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
					}
					slog.Error("Device authorization error", "error", errorResp.Error)
					return nil, fmt.Errorf("device authorization error: %s", errorResp.Error)
				}
			}
		}
		slog.Error("Device authorization failed", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("device authorization request failed with status %d", resp.StatusCode)
	}

	// Parse the response
	var deviceAuthResp DeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceAuthResp); err != nil {
		slog.Error("Failed to parse device authorization response", "error", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	slog.Debug("Device authorization response parsed",
		"device_code_present", deviceAuthResp.DeviceCode != "",
		"user_code_present", deviceAuthResp.UserCode != "",
		"verification_uri_present", deviceAuthResp.VerificationURI != "",
		"verification_uri_complete_present", deviceAuthResp.VerificationURIComplete != "",
		"expires_in", deviceAuthResp.ExpiresIn,
		"interval", deviceAuthResp.Interval)

	// Validate required fields in response
	if deviceAuthResp.DeviceCode == "" || deviceAuthResp.UserCode == "" || deviceAuthResp.VerificationURI == "" {
		slog.Error("Device authorization response missing required fields")
		return nil, fmt.Errorf("device authorization response missing required fields")
	}

	// Set default interval if not provided
	if deviceAuthResp.Interval <= 0 {
		deviceAuthResp.Interval = 5 // Default to 5 seconds
		slog.Debug("Setting default polling interval", "interval", deviceAuthResp.Interval)
	}

	// Set default expiration if not provided
	if deviceAuthResp.ExpiresIn <= 0 {
		deviceAuthResp.ExpiresIn = 1800 // Default to 30 minutes
		slog.Debug("Setting default expiration time", "expires_in", deviceAuthResp.ExpiresIn)
	}

	slog.Info("Device authorization successful",
		"user_code", deviceAuthResp.UserCode,
		"verification_uri", deviceAuthResp.VerificationURI,
		"expires_in", deviceAuthResp.ExpiresIn,
		"interval", deviceAuthResp.Interval)

	return &deviceAuthResp, nil
}

func (q *Qwen3OAuthComponent) ExchangeCodeForToken(code string) tea.Cmd {
	// This exchanges the device code for an access token
	return func() tea.Msg {
		// In a real implementation, this would:
		// 1. Make an HTTP POST request to the token endpoint
		// 2. Include the device code, client ID, and code verifier
		// 3. Receive the access token and refresh token
		// 4. Store the tokens securely
		// For now, we'll simulate the token exchange with a delay
		time.Sleep(1 * time.Second)
		// Simulate successful token exchange
		// In a real implementation, this would return the actual tokens
		return Qwen3OAuthStateChangeMsg{
			State: Qwen3OAuthStateSuccess,
		}
	}
}

func (q *Qwen3OAuthComponent) ID() dialogs.DialogID {
	return "qwen3-oauth"
}

// Close implements the CloseCallback interface
func (q *Qwen3OAuthComponent) Close() tea.Cmd {
	// Reset state when dialog is closed
	q.state = Qwen3OAuthStateInitial
	q.errorMessage = ""
	q.successMessage = ""
	q.deviceAuthResp = nil
	q.codeVerifier = ""
	q.pollInterval = 2 * time.Second
	q.maxAttempts = 30
	q.attemptCount = 0
	return nil
}

// Ensure Qwen3OAuthComponent implements the OAuthDialog and CloseCallback interfaces
var _ OAuthDialog = (*Qwen3OAuthComponent)(nil)
var _ dialogs.CloseCallback = (*Qwen3OAuthComponent)(nil)
