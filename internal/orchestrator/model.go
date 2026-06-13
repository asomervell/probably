package orchestrator

import (
	"fmt"
	"strings"
)

// Provider represents a supported LLM provider
type Provider string

const (
	ProviderXAI       Provider = "xai"       // xAI (Grok)
	ProviderGoogle    Provider = "google"    // Google AI (Gemini, Gemma)
	ProviderGroq      Provider = "groq"      // Groq
	ProviderAnthropic Provider = "anthropic" // Anthropic (Claude)
)

// ModelRole defines what the model is used for
type ModelRole string

const (
	RoleFast      ModelRole = "fast"      // Quick, cheap responses
	RoleReasoning ModelRole = "reasoning" // Deep thinking, complex tasks
	RoleToolCall  ModelRole = "tool_call" // Function calling capable
	RoleVerifier  ModelRole = "verifier"  // Checks other models' work
	RoleVision    ModelRole = "vision"    // Multimodal: images, PDFs, video
	RolePlanner   ModelRole = "planner"   // Supervisor strategy: task decomposition
)

// ModelSpec represents a parsed provider/model specification
type ModelSpec struct {
	Provider Provider
	Model    string
	Role     ModelRole
}

// ParseModelSpec parses a "provider/model" string into a ModelSpec
func ParseModelSpec(spec string) (*ModelSpec, error) {
	if spec == "" {
		return nil, fmt.Errorf("empty model spec")
	}

	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model spec %q: expected 'provider/model' format", spec)
	}

	provider := Provider(strings.ToLower(parts[0]))
	model := parts[1]

	switch provider {
	case ProviderXAI, ProviderGoogle, ProviderGroq, ProviderAnthropic:
		// Valid provider
	default:
		return nil, fmt.Errorf("unknown provider %q: supported providers are xai, google, groq, anthropic", provider)
	}

	return &ModelSpec{Provider: provider, Model: model}, nil
}

// String returns the string representation of the model spec
func (m *ModelSpec) String() string {
	return string(m.Provider) + "/" + m.Model
}
