package message

import (
	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

// ModelsUpdateMsg is a message that is sent when the list of available models is updated
type ModelsUpdateMsg struct {
	ProviderID string
	Models     []catwalk.Model
}