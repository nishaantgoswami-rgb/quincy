package message

import (
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/assert"
)

func TestModelsUpdateMsg(t *testing.T) {
	// Create a test message
	providerID := "test-provider"
	models := []catwalk.Model{
		{
			ID:                     "test-model-1",
			Name:                   "Test Model 1",
			ContextWindow:          128000,
			DefaultMaxTokens:       4096,
			CanReason:              true,
			HasReasoningEffort:     true,
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
		{
			ID:                     "test-model-2",
			Name:                   "Test Model 2",
			ContextWindow:          200000,
			DefaultMaxTokens:       4096,
			CanReason:              true,
			HasReasoningEffort:     true,
			DefaultReasoningEffort: "high",
			SupportsImages:         false,
		},
	}

	msg := ModelsUpdateMsg{
		ProviderID: providerID,
		Models:     models,
	}

	// Verify the message fields
	assert.Equal(t, providerID, msg.ProviderID)
	assert.Equal(t, len(models), len(msg.Models))
	assert.Equal(t, models[0].ID, msg.Models[0].ID)
	assert.Equal(t, models[1].ID, msg.Models[1].ID)
	assert.Equal(t, models[0].Name, msg.Models[0].Name)
	assert.Equal(t, models[1].Name, msg.Models[1].Name)
	assert.Equal(t, models[0].ContextWindow, msg.Models[0].ContextWindow)
	assert.Equal(t, models[1].ContextWindow, msg.Models[1].ContextWindow)
}