package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/tidwall/sjson"
)

const (
	appName              = "crush"
	defaultDataDirectory = ".crush"
)

// Qwen OAuth Configuration
const (
	QwenOAuthBaseURL        = "https://chat.qwen.ai"
	QwenTokenEndpoint       = QwenOAuthBaseURL + "/api/v1/oauth2/token"
	QwenDefaultClientID     = "f0304373b74a44d2b584a3fb70ca9e56" // OAuth 2.0 public client ID (safe to embed per RFC 8628)
	QwenRefreshGrantType    = "refresh_token"
)

// getQwenClientID returns the Qwen OAuth client ID, checking environment variable first.
func getQwenClientID() string {
	if clientID := os.Getenv("QWEN_CLIENT_ID"); clientID != "" {
		return clientID
	}
	return QwenDefaultClientID
}

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"GEMINI.md",
	"gemini.md",
	"crush.md",
	"crush.local.md",
	"Crush.md",
	"Crush.local.md",
	"CRUSH.md",
	"CRUSH.local.md",
	"AGENTS.md",
	"agents.md",
	"Agents.md",
}

type SelectedModelType string

const (
	SelectedModelTypeLarge SelectedModelType = "large"
	SelectedModelTypeSmall SelectedModelType = "small"
)

type SelectedModel struct {
	// The model id as used by the provider API.
	// Required.
	Model string `json:"model" jsonschema:"required,description=The model ID as used by the provider API,example=gpt-4o"`
	// The model provider, same as the key/id used in the providers config.
	// Required.
	Provider string `json:"provider" jsonschema:"required,description=The model provider ID that matches a key in the providers config,example=openai"`

	// Only used by models that use the openai provider and need this set.
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"description=Reasoning effort level for OpenAI models that support it,enum=low,enum=medium,enum=high"`

	// Overrides the default model configuration.
	MaxTokens int64 `json:"max_tokens,omitempty" jsonschema:"description=Maximum number of tokens for model responses,minimum=1,maximum=200000,example=4096"`

	// Used by anthropic models that can reason to indicate if the model should think.
	Think bool `json:"think,omitempty" jsonschema:"description=Enable thinking mode for Anthropic models that support reasoning"`
}

type ProviderConfig struct {
	// The provider's id.
	ID string `json:"id,omitempty" jsonschema:"description=Unique identifier for the provider,example=openai"`
	// The provider's name, used for display purposes.
	Name string `json:"name,omitempty" jsonschema:"description=Human-readable name for the provider,example=OpenAI"`
	// The provider's API endpoint.
	BaseURL string `json:"base_url,omitempty" jsonschema:"description=Base URL for the provider's API,format=uri,example=https://api.openai.com/v1"`
	// The provider type, e.g. "openai", "anthropic", etc. if empty it defaults to openai.
	Type catwalk.Type `json:"type,omitempty" jsonschema:"description=Provider type that determines the API format,enum=openai,enum=anthropic,enum=gemini,enum=azure,enum=vertexai,default=openai"`
	// The provider's API key.
	APIKey string `json:"api_key,omitempty" jsonschema:"description=API key for authentication with the provider,example=$OPENAI_API_KEY"`
	// OAuth access token for providers that support OAuth authentication
	OAuthToken string `json:"oauth_token,omitempty" jsonschema:"description=OAuth access token for providers that support OAuth authentication"`
	// OAuth refresh token for providers that support OAuth authentication
	OAuthRefresh string `json:"oauth_refresh,omitempty" jsonschema:"description=OAuth refresh token for providers that support OAuth authentication"`
	// OAuth token expiry time (Unix timestamp)
	OAuthExpiry int64 `json:"oauth_expiry,omitempty" jsonschema:"description=OAuth token expiry time (Unix timestamp)"`
	// Authentication type (api_key or oauth)
	AuthType string `json:"auth_type,omitempty" jsonschema:"description=Authentication type (api_key or oauth),enum=api_key,enum=oauth"`
	// Marks the provider as disabled.
	Disable bool `json:"disable,omitempty" jsonschema:"description=Whether this provider is disabled,default=false"`

	// Custom system prompt prefix.
	SystemPromptPrefix string `json:"system_prompt_prefix,omitempty" jsonschema:"description=Custom prefix to add to system prompts for this provider"`

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty" jsonschema:"description=Additional HTTP headers to send with requests"`
	// Extra body
	ExtraBody map[string]any `json:"extra_body,omitempty" jsonschema:"description=Additional fields to include in request bodies"`

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// The provider models
	Models []catwalk.Model `json:"models,omitempty" jsonschema:"description=List of models available from this provider"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command  string            `json:"command,omitempty" jsonschema:"description=Command to execute for stdio MCP servers,example=npx"`
	Env      map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the MCP server"`
	Args     []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the MCP server command"`
	Type     MCPType           `json:"type" jsonschema:"required,description=Type of MCP connection,enum=stdio,enum=sse,enum=http,default=stdio"`
	URL      string            `json:"url,omitempty" jsonschema:"description=URL for HTTP or SSE MCP servers,format=uri,example=http://localhost:3000/mcp"`
	Disabled bool              `json:"disabled,omitempty" jsonschema:"description=Whether this MCP server is disabled,default=false"`
	Timeout  int               `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for MCP server connections,default=15,example=30,example=60,example=120"`

	// TODO: maybe make it possible to get the value from the env
	Headers map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers for HTTP/SSE MCP servers"`
}

type LSPConfig struct {
	Disabled    bool              `json:"disabled,omitempty" jsonschema:"description=Whether this LSP server is disabled,default=false"`
	Command     string            `json:"command" jsonschema:"required,description=Command to execute for the LSP server,example=gopls"`
	Args        []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the LSP server command"`
	Env         map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set to the LSP server command"`
	FileTypes   []string          `json:"filetypes,omitempty" jsonschema:"description=File types this LSP server handles,example=go,example=mod,example=rs,example=c,example=js,example=ts"`
	RootMarkers []string          `json:"root_markers,omitempty" jsonschema:"description=Files or directories that indicate the project root,example=go.mod,example=package.json,example=Cargo.toml"`
	InitOptions map[string]any    `json:"init_options,omitempty" jsonschema:"description=Initialization options passed to the LSP server during initialize request"`
	Options     map[string]any    `json:"options,omitempty" jsonschema:"description=LSP server-specific settings passed during initialization"`
}

type TUIOptions struct {
	CompactMode bool   `json:"compact_mode,omitempty" jsonschema:"description=Enable compact mode for the TUI interface,default=false"`
	DiffMode    string `json:"diff_mode,omitempty" jsonschema:"description=Diff mode for the TUI interface,enum=unified,enum=split"`
	// Here we can add themes later or any TUI related options
}

type Permissions struct {
	AllowedTools []string `json:"allowed_tools,omitempty" jsonschema:"description=List of tools that don't require permission prompts,example=bash,example=view"` // Tools that don't require permission prompts
	SkipRequests bool     `json:"-"`                                                                                                                              // Automatically accept all permissions (YOLO mode)
}

type Attribution struct {
	CoAuthoredBy  bool `json:"co_authored_by,omitempty" jsonschema:"description=Add Co-Authored-By trailer to commit messages,default=true"`
	GeneratedWith bool `json:"generated_with,omitempty" jsonschema:"description=Add Generated with Crush line to commit messages and issues and PRs,default=true"`
}

type Options struct {
	ContextPaths              []string     `json:"context_paths,omitempty" jsonschema:"description=Paths to files containing context information for the AI,example=.cursorrules,example=CRUSH.md"`
	TUI                       *TUIOptions  `json:"tui,omitempty" jsonschema:"description=Terminal user interface options"`
	Debug                     bool         `json:"debug,omitempty" jsonschema:"description=Enable debug logging,default=false"`
	DebugLSP                  bool         `json:"debug_lsp,omitempty" jsonschema:"description=Enable debug logging for LSP servers,default=false"`
	DisableAutoSummarize      bool         `json:"disable_auto_summarize,omitempty" jsonschema:"description=Disable automatic conversation summarization,default=false"`
	DataDirectory             string       `json:"data_directory,omitempty" jsonschema:"description=Directory for storing application data (relative to working directory),default=.crush,example=.crush"` // Relative to the cwd
	DisabledTools             []string     `json:"disabled_tools" jsonschema:"description=Tools to disable"`
	DisableProviderAutoUpdate bool         `json:"disable_provider_auto_update,omitempty" jsonschema:"description=Disable providers auto-update,default=false"`
	Attribution               *Attribution `json:"attribution,omitempty" jsonschema:"description=Attribution settings for generated content"`
}

type MCPs map[string]MCPConfig

type MCP struct {
	Name string    `json:"name"`
	MCP  MCPConfig `json:"mcp"`
}

func (m MCPs) Sorted() []MCP {
	sorted := make([]MCP, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, MCP{
			Name: k,
			MCP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b MCP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

type LSPs map[string]LSPConfig

type LSP struct {
	Name string    `json:"name"`
	LSP  LSPConfig `json:"lsp"`
}

func (l LSPs) Sorted() []LSP {
	sorted := make([]LSP, 0, len(l))
	for k, v := range l {
		sorted = append(sorted, LSP{
			Name: k,
			LSP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b LSP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

func (l LSPConfig) ResolvedEnv() []string {
	return resolveEnvs(l.Env)
}

func (m MCPConfig) ResolvedEnv() []string {
	return resolveEnvs(m.Env)
}

func (m MCPConfig) ResolvedHeaders() map[string]string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range m.Headers {
		var err error
		m.Headers[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving header variable", "error", err, "variable", e, "value", v)
			continue
		}
	}
	return m.Headers
}

type Agent struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// This is the id of the system prompt used by the agent
	Disabled bool `json:"disabled,omitempty"`

	Model SelectedModelType `json:"model" jsonschema:"required,description=The model type to use for this agent,enum=large,enum=small,default=large"`

	// The available tools for the agent
	//  if this is nil, all tools are available
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the AllowedMCP the agent has available
	//  if the string array is nil, all tools from the AllowedMCP are available
	AllowedMCP map[string][]string `json:"allowed_mcp,omitempty"`

	// The list of LSPs that this agent can use
	//  if this is nil, all LSPs are available
	AllowedLSP []string `json:"allowed_lsp,omitempty"`

	// Overrides the context paths for this agent
	ContextPaths []string `json:"context_paths,omitempty"`
}

// Config holds the configuration for crush.
type Config struct {
	Schema string `json:"$schema,omitempty"`

	// We currently only support large/small as values here.
	Models map[SelectedModelType]SelectedModel `json:"models,omitempty" jsonschema:"description=Model configurations for different model types,example={\"large\":{\"model\":\"gpt-4o\",\"provider\":\"openai\"}}"`

	// The providers that are configured
	Providers *csync.Map[string, ProviderConfig] `json:"providers,omitempty" jsonschema:"description=AI provider configurations"`

	MCP MCPs `json:"mcp,omitempty" jsonschema:"description=Model Context Protocol server configurations"`

	LSP LSPs `json:"lsp,omitempty" jsonschema:"description=Language Server Protocol configurations"`

	Options *Options `json:"options,omitempty" jsonschema:"description=General application options"`

	Permissions *Permissions `json:"permissions,omitempty" jsonschema:"description=Permission settings for tool usage"`

	// Internal
	workingDir string `json:"-"`
	// TODO: most likely remove this concept when I come back to it
	Agents map[string]Agent `json:"-"`
	// TODO: find a better way to do this this should probably not be part of the config
	resolver       VariableResolver
	dataConfigDir  string             `json:"-"`
	knownProviders []catwalk.Provider `json:"-"`
}

func (c *Config) WorkingDir() string {
	return c.workingDir
}

func (c *Config) EnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
	for p := range c.Providers.Seq() {
		if !p.Disable {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// IsConfigured  return true if at least one provider is configured
func (c *Config) IsConfigured() bool {
	return len(c.EnabledProviders()) > 0
}

// refreshQwen3Token attempts to refresh an expired Qwen OAuth token using the refresh token.
func (c *Config) refreshQwen3Token(providerID string, refreshToken string) error {
	slog.Info("Attempting to refresh Qwen OAuth token", "provider", providerID)

	// Prepare the token refresh request
	data := url.Values{}
	data.Set("grant_type", QwenRefreshGrantType)
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", getQwenClientID())

	slog.Debug("Prepared token refresh request", "grant_type", QwenRefreshGrantType)

	// Create the HTTP request
	req, err := http.NewRequest("POST", QwenTokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		slog.Error("Failed to create token refresh request", "error", err)
		return fmt.Errorf("failed to create token refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	slog.Debug("Sending token refresh request to Qwen", "endpoint", QwenTokenEndpoint)

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Token refresh request failed", "error", err)
		return fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("Received token refresh response", "status_code", resp.StatusCode)

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Error("Failed to read error response body", "error", readErr)
		} else {
			slog.Debug("Token refresh error response body", "body", string(bodyBytes))
			var errorResp struct {
				Error            string `json:"error"`
				ErrorDescription string `json:"error_description"`
			}
			if decodeErr := json.Unmarshal(bodyBytes, &errorResp); decodeErr == nil && errorResp.Error != "" {
				if errorResp.ErrorDescription != "" {
					slog.Error("Token refresh error", "error", errorResp.Error, "description", errorResp.ErrorDescription)
					return fmt.Errorf("token refresh failed: %s - %s", errorResp.Error, errorResp.ErrorDescription)
				}
				slog.Error("Token refresh error", "error", errorResp.Error)
				return fmt.Errorf("token refresh failed: %s", errorResp.Error)
			}
		}
		slog.Error("Token refresh failed", "status_code", resp.StatusCode)
		return fmt.Errorf("token refresh request failed with status %d", resp.StatusCode)
	}

	// Parse the successful response
	type TokenResponse struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error,omitempty"`
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		slog.Error("Failed to parse token refresh response", "error", err)
		return fmt.Errorf("failed to parse token refresh response: %w", err)
	}

	slog.Debug("Token refresh response parsed",
		"access_token_present", tokenResp.AccessToken != "",
		"refresh_token_present", tokenResp.RefreshToken != "",
		"token_type", tokenResp.TokenType,
		"expires_in", tokenResp.ExpiresIn)

	// Verify we got the required tokens
	if tokenResp.AccessToken == "" {
		slog.Error("Token refresh succeeded but no access token was returned")
		return fmt.Errorf("token refresh succeeded but no access token was returned")
	}

	// If no new refresh token was provided, use the old one
	newRefreshToken := tokenResp.RefreshToken
	if newRefreshToken == "" {
		slog.Debug("No new refresh token provided, using existing refresh token")
		newRefreshToken = refreshToken
	}

	// Calculate new expiry time
	currentTime := time.Now().Unix()
	newExpiry := currentTime + int64(tokenResp.ExpiresIn)

	// Save the new tokens
	err = c.SetProviderCredentials(providerID, "", tokenResp.AccessToken, newRefreshToken, newExpiry, "oauth")
	if err != nil {
		slog.Error("Failed to save refreshed OAuth tokens", "error", err)
		return fmt.Errorf("failed to save refreshed OAuth tokens: %w", err)
	}

	slog.Info("Token refresh successful",
		"access_token_length", len(tokenResp.AccessToken),
		"token_type", tokenResp.TokenType,
		"refresh_token_present", tokenResp.RefreshToken != "",
		"expires_in", tokenResp.ExpiresIn)

	return nil
}

// IsQwen3CoderAuthenticated checks if the Qwen3 Coder provider is authenticated
// by verifying if it has a valid OAuth token that hasn't expired yet.
// If the token has expired but a refresh token is available, it attempts to refresh the token.
func (c *Config) IsQwen3CoderAuthenticated() bool {
	// Get the Qwen3 Coder provider configuration
	providerConfig, exists := c.Providers.Get("qwen3-coder-oauth")
	if !exists {
		return false
	}

	// Check if the provider is disabled
	if providerConfig.Disable {
		return false
	}

	// For OAuth authentication, check if we have a token and if it's valid
	if providerConfig.AuthType == "oauth" && providerConfig.OAuthToken != "" {
		// If we have an expiry time, check if the token is still valid
		if providerConfig.OAuthExpiry > 0 {
			// Check if the token has expired
			if providerConfig.OAuthExpiry <= time.Now().Unix() {
				// Token has expired, try to refresh it if we have a refresh token
				if providerConfig.OAuthRefresh != "" {
					slog.Info("OAuth token expired, attempting refresh", "provider", "qwen3-coder-oauth")
					err := c.refreshQwen3Token("qwen3-coder-oauth", providerConfig.OAuthRefresh)
					if err != nil {
						slog.Error("Failed to refresh OAuth token", "error", err)
						return false // Refresh failed
					}
					// Refresh succeeded, the token is now valid
					return true
				}
				// No refresh token available
				slog.Warn("OAuth token expired and no refresh token available")
				return false
			}
		}
		// Token exists and either has no expiry or hasn't expired yet
		return true
	}

	// If we get here, the provider is configured but not properly authenticated
	return false
}

// IsProviderAuthenticated checks if a provider is authenticated.
// For Qwen3 Coder, it uses the specialized OAuth check.
// For other providers, it checks if they have an API key.
func (c *Config) IsProviderAuthenticated(providerID string) bool {
	// Special handling for Qwen3 Coder which uses OAuth
	if providerID == "qwen3-coder-oauth" {
		return c.IsQwen3CoderAuthenticated()
	}

	// For other providers, check if they have an API key
	providerConfig, exists := c.Providers.Get(providerID)
	if !exists {
		return false
	}

	// Check if the provider is disabled
	if providerConfig.Disable {
		return false
	}

	// Check if the provider has an API key
	return providerConfig.APIKey != ""
}

func (c *Config) GetModel(provider, model string) *catwalk.Model {
	if providerConfig, ok := c.Providers.Get(provider); ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

func (c *Config) GetProviderForModel(modelType SelectedModelType) *ProviderConfig {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	if providerConfig, ok := c.Providers.Get(model.Provider); ok {
		return &providerConfig
	}
	return nil
}

func (c *Config) GetModelByType(modelType SelectedModelType) *catwalk.Model {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) LargeModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeLarge]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SmallModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeSmall]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SetCompactMode(enabled bool) error {
	if c.Options == nil {
		c.Options = &Options{}
	}
	c.Options.TUI.CompactMode = enabled
	return c.SetConfigField("options.tui.compact_mode", enabled)
}

func (c *Config) Resolve(key string) (string, error) {
	if c.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return c.resolver.ResolveValue(key)
}

func (c *Config) UpdatePreferredModel(modelType SelectedModelType, model SelectedModel) error {
	c.Models[modelType] = model
	if err := c.SetConfigField(fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	return nil
}

func (c *Config) SetConfigField(key string, value any) error {
	// read the data
	data, err := os.ReadFile(c.dataConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	newValue, err := sjson.Set(string(data), key, value)
	if err != nil {
		return fmt.Errorf("failed to set config field %s: %w", key, err)
	}
	if err := os.WriteFile(c.dataConfigDir, []byte(newValue), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func (c *Config) SetProviderAPIKey(providerID, apiKey string) error {
	return c.SetProviderCredentials(providerID, apiKey, "", "", 0, "api_key")
}

func (c *Config) SetProviderCredentials(providerID, apiKey, oauthToken, oauthRefresh string, oauthExpiry int64, authType string) error {
	// Save to the config file
	var err error
	if authType == "oauth" {
		err = c.SetConfigField("providers."+providerID+".oauth_token", oauthToken)
		if err != nil {
			return fmt.Errorf("failed to save OAuth token to config file: %w", err)
		}
		if oauthRefresh != "" {
			err = c.SetConfigField("providers."+providerID+".oauth_refresh", oauthRefresh)
			if err != nil {
				return fmt.Errorf("failed to save OAuth refresh token to config file: %w", err)
			}
		}
		if oauthExpiry > 0 {
			err = c.SetConfigField("providers."+providerID+".oauth_expiry", oauthExpiry)
			if err != nil {
				return fmt.Errorf("failed to save OAuth expiry to config file: %w", err)
			}
		}
		err = c.SetConfigField("providers."+providerID+".auth_type", authType)
		if err != nil {
			return fmt.Errorf("failed to save auth type to config file: %w", err)
		}
	} else {
		err = c.SetConfigField("providers."+providerID+".api_key", apiKey)
		if err != nil {
			return fmt.Errorf("failed to save API key to config file: %w", err)
		}
	}

	providerConfig, exists := c.Providers.Get(providerID)
	if exists {
		if authType == "oauth" {
			providerConfig.OAuthToken = oauthToken
			providerConfig.OAuthRefresh = oauthRefresh
			providerConfig.OAuthExpiry = oauthExpiry
			providerConfig.AuthType = authType
		} else {
			providerConfig.APIKey = apiKey
		}
		c.Providers.Set(providerID, providerConfig)
		return nil
	}

	var foundProvider *catwalk.Provider
	for _, p := range c.knownProviders {
		if string(p.ID) == providerID {
			foundProvider = &p
			break
		}
	}

	if foundProvider != nil {
		// Create new provider config based on known provider
		providerConfig = ProviderConfig{
			ID:           providerID,
			Name:         foundProvider.Name,
			BaseURL:      foundProvider.APIEndpoint,
			Type:         foundProvider.Type,
			APIKey:       apiKey,
			OAuthToken:   oauthToken,
			OAuthRefresh: oauthRefresh,
			OAuthExpiry:  oauthExpiry,
			AuthType:     authType,
			Disable:      false,
			ExtraHeaders: make(map[string]string),
			ExtraParams:  make(map[string]string),
			Models:       foundProvider.Models,
		}
	} else {
		return fmt.Errorf("provider with ID %s not found in known providers", providerID)
	}
	// Store the updated provider config
	c.Providers.Set(providerID, providerConfig)
	return nil
}

func allToolNames() []string {
	return []string{
		"bash",
		"download",
		"edit",
		"multiedit",
		"fetch",
		"glob",
		"grep",
		"ls",
		"sourcegraph",
		"view",
		"write",
	}
}

func resolveAllowedTools(allTools []string, disabledTools []string) []string {
	if disabledTools == nil {
		return allTools
	}
	// filter out disabled tools (exclude mode)
	return filterSlice(allTools, disabledTools, false)
}

func resolveReadOnlyTools(tools []string) []string {
	readOnlyTools := []string{"glob", "grep", "ls", "sourcegraph", "view"}
	// filter to only include tools that are in allowedtools (include mode)
	return filterSlice(tools, readOnlyTools, true)
}

func filterSlice(data []string, mask []string, include bool) []string {
	filtered := []string{}
	for _, s := range data {
		// if include is true, we include items that ARE in the mask
		// if include is false, we include items that are NOT in the mask
		if include == slices.Contains(mask, s) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func (c *Config) SetupAgents() {
	allowedTools := resolveAllowedTools(allToolNames(), c.Options.DisabledTools)

	agents := map[string]Agent{
		"coder": {
			ID:           "coder",
			Name:         "Coder",
			Description:  "An agent that helps with executing coding tasks.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: c.Options.ContextPaths,
			AllowedTools: allowedTools,
		},
		"task": {
			ID:           "task",
			Name:         "Task",
			Description:  "An agent that helps with searching for context and finding implementation details.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: c.Options.ContextPaths,
			AllowedTools: resolveReadOnlyTools(allowedTools),
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			AllowedLSP: []string{},
		},
	}
	c.Agents = agents
}

func (c *Config) Resolver() VariableResolver {
	return c.resolver
}

func (c *ProviderConfig) TestConnection(resolver VariableResolver) error {
	testURL := ""
	headers := make(map[string]string)
	
	// Handle OAuth vs API key authentication
	var token string
	if c.AuthType == "oauth" && c.OAuthToken != "" {
		token = c.OAuthToken
	} else {
		token, _ = resolver.ResolveValue(c.APIKey)
	}
	
	switch c.Type {
	case catwalk.TypeOpenAI:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if c.ID == string(catwalk.InferenceProviderOpenRouter) {
			testURL = baseURL + "/credits"
		} else {
			testURL = baseURL + "/models"
		}
		if c.AuthType == "oauth" && c.OAuthToken != "" {
			headers["Authorization"] = "Bearer " + c.OAuthToken
		} else {
			headers["Authorization"] = "Bearer " + token
		}
	case catwalk.TypeAnthropic:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		testURL = baseURL + "/models"
		if c.AuthType == "oauth" && c.OAuthToken != "" {
			headers["Authorization"] = "Bearer " + c.OAuthToken
		} else {
			headers["x-api-key"] = token
		}
		headers["anthropic-version"] = "2023-06-01"
	case catwalk.TypeGemini:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		if c.AuthType == "oauth" && c.OAuthToken != "" {
			testURL = baseURL + "/v1beta/models?key=" + url.QueryEscape(c.OAuthToken)
		} else {
			testURL = baseURL + "/v1beta/models?key=" + url.QueryEscape(token)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}
	b, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	if c.ID == string(catwalk.InferenceProviderZAI) {
		if b.StatusCode == http.StatusUnauthorized {
			// for z.ai just check if the http response is not 401
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, b.Status)
		}
	} else {
		if b.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, b.Status)
		}
	}
	_ = b.Body.Close()
	return nil
}

func resolveEnvs(envs map[string]string) []string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range envs {
		var err error
		envs[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving environment variable", "error", err, "variable", e, "value", v)
			continue
		}
	}

	res := make([]string, 0, len(envs))
	for k, v := range envs {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return res
}
