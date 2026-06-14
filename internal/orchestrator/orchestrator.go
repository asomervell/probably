package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/observability"
	"google.golang.org/genai"
)

// Orchestrator coordinates LLM requests across multiple models and strategies
type Orchestrator struct {
	cfg *config.Config

	// Model pool by role
	models map[ModelRole]*ModelSpec

	// Provider endpoints
	endpoints map[Provider]string

	// HTTP client
	httpClient *http.Client

	// Vertex AI configuration (if using Vertex instead of Google AI Studio)
	useVertex      bool
	vertexProject  string
	vertexLocation string
	vertexClient   *genai.Client // GenAI SDK client for Vertex AI
	geminiClient   *genai.Client // GenAI SDK client for Gemini Direct API

	// Strategy implementations
	strategies map[Strategy]StrategyExecutor
}

// StrategyExecutor defines the interface for strategy implementations
type StrategyExecutor interface {
	Execute(ctx context.Context, task *Task, orch *Orchestrator) (*Result, error)
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(cfg *config.Config) (*Orchestrator, error) {
	orch := &Orchestrator{
		cfg: cfg,
		endpoints: map[Provider]string{
			ProviderXAI:       "https://api.x.ai/v1",
			ProviderGoogle:    "https://generativelanguage.googleapis.com/v1beta/openai",
			ProviderGroq:      "https://api.groq.com/openai/v1",
			ProviderAnthropic: "https://api.anthropic.com/v1",
		},
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		models:     make(map[ModelRole]*ModelSpec),
		strategies: make(map[Strategy]StrategyExecutor),
	}

	// Parse default model (required)
	if cfg.LLMDefaultModel == "" {
		return nil, fmt.Errorf("LLM_DEFAULT_MODEL is required (e.g., 'google/gemini-2.0-flash')")
	}

	defaultSpec, err := ParseModelSpec(cfg.LLMDefaultModel)
	if err != nil {
		return nil, fmt.Errorf("invalid LLM_DEFAULT_MODEL: %w", err)
	}
	defaultSpec.Role = RoleFast
	orch.models[RoleFast] = defaultSpec

	// Check if using Vertex AI
	if defaultSpec.Provider == ProviderGoogle && cfg.VertexProject != "" {
		orch.useVertex = true
		orch.vertexProject = cfg.VertexProject
		orch.vertexLocation = cfg.VertexLocation
		if orch.vertexLocation == "" {
			orch.vertexLocation = "us-central1"
		}

		ctx := context.Background()
		orch.vertexClient, err = genai.NewClient(ctx, &genai.ClientConfig{
			Project:  orch.vertexProject,
			Location: orch.vertexLocation,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
		}

		// Vertex AI configured successfully
	}

	// Initialize Gemini Direct API client when not using Vertex AI
	if !orch.useVertex && cfg.GoogleAPIKey != "" {
		ctx := context.Background()
		orch.geminiClient, err = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GoogleAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini API client: %w", err)
		}
	}

	// Parse reasoning model (optional)
	if cfg.LLMReasoningModel != "" {
		reasoningSpec, err := ParseModelSpec(cfg.LLMReasoningModel)
		if err != nil {
			return nil, fmt.Errorf("invalid LLM_REASONING_MODEL: %w", err)
		}
		reasoningSpec.Role = RoleReasoning
		orch.models[RoleReasoning] = reasoningSpec
	} else {
		// Create a copy with correct role for fallback (allocate on heap)
		fallbackSpec := &ModelSpec{
			Provider: defaultSpec.Provider,
			Model:    defaultSpec.Model,
			Role:     RoleReasoning,
		}
		orch.models[RoleReasoning] = fallbackSpec
	}

	// Parse tool call model (optional)
	if cfg.LLMToolCallModel != "" {
		toolCallSpec, err := ParseModelSpec(cfg.LLMToolCallModel)
		if err != nil {
			return nil, fmt.Errorf("invalid LLM_TOOL_CALL_MODEL: %w", err)
		}
		toolCallSpec.Role = RoleToolCall
		orch.models[RoleToolCall] = toolCallSpec
	} else {
		// Create a copy with correct role for fallback (allocate on heap)
		fallbackSpec := &ModelSpec{
			Provider: defaultSpec.Provider,
			Model:    defaultSpec.Model,
			Role:     RoleToolCall,
		}
		orch.models[RoleToolCall] = fallbackSpec
	}

	// Vision (statement/document OCR) is supported via Anthropic (Messages API)
	// and Google (Vertex AI / Gemini API). Reuse the default model when it is
	// vision-capable; otherwise fall back to whichever vision provider is keyed,
	// preferring Anthropic. With no vision-capable provider, vision stays
	// unregistered and statement OCR degrades gracefully (see SupportsVision).
	switch {
	case defaultSpec.Provider == ProviderAnthropic || defaultSpec.Provider == ProviderGoogle:
		orch.models[RoleVision] = &ModelSpec{
			Provider: defaultSpec.Provider,
			Model:    defaultSpec.Model,
			Role:     RoleVision,
		}
	case cfg.AnthropicAPIKey != "":
		orch.models[RoleVision] = &ModelSpec{
			Provider: ProviderAnthropic,
			Model:    cfg.ClaudeModel,
			Role:     RoleVision,
		}
	case cfg.GoogleAPIKey != "":
		orch.models[RoleVision] = &ModelSpec{
			Provider: ProviderGoogle,
			Model:    "gemini-2.5-flash",
			Role:     RoleVision,
		}
	}

	// Initialize strategies
	orch.strategies[StrategySimple] = &SimpleStrategy{}
	orch.strategies[StrategyEscalate] = &EscalateStrategy{}
	orch.strategies[StrategyConsensus] = &ConsensusStrategy{}
	orch.strategies[StrategyVerify] = &VerifyStrategy{}
	orch.strategies[StrategySupervisor] = &SupervisorStrategy{}

	// Orchestrator initialized - logs are handled by the worker that creates it

	return orch, nil
}

// Execute runs a task using the specified strategy (or default)
func (o *Orchestrator) Execute(ctx context.Context, task *Task) (*Result, error) {
	if !o.IsConfigured() {
		return nil, fmt.Errorf("orchestrator not configured")
	}

	// Use default strategy if not specified
	strategy := task.Strategy
	if strategy == "" {
		// Get default strategy from config
		configStrategy := o.cfg.LLMDefaultStrategy
		if configStrategy != "" {
			strategy = Strategy(configStrategy)
		} else {
			strategy = StrategySimple // Fallback default
		}
	}

	executor, ok := o.strategies[strategy]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	// Get model info for tracing
	model := o.GetModel(RoleFast)
	modelName := ""
	if model != nil {
		modelName = model.Model
	}

	// Start agent span for the entire task execution
	// This follows Sentry's gen_ai.invoke_agent convention
	agentName := fmt.Sprintf("%s_%s", task.Type, strategy)
	agentSpan := observability.StartAgent(ctx, observability.AgentOptions{
		Name:        agentName,
		Model:       modelName,
		Description: fmt.Sprintf("AI task: %s with strategy %s", task.Type, strategy),
		Tags: map[string]string{
			"task_type": string(task.Type),
			"strategy":  string(strategy),
		},
	})
	agentSpan.SetData("gen_ai.operation.name", "invoke_agent")
	if task.Context != nil {
		agentSpan.SetData("ledger_id", task.Context.LedgerID)
	}
	agentCtx := agentSpan.Context()

	slog.InfoContext(agentCtx, "Starting AI agent",
		"agent_name", agentName,
		"task_type", string(task.Type),
		"strategy", string(strategy),
		"model", modelName,
	)

	startTime := time.Now()
	result, err := executor.Execute(agentCtx, task, o)
	duration := time.Since(startTime)

	success := err == nil
	confidence := 0.0
	if result != nil {
		confidence = result.Confidence
		agentSpan.SetData("gen_ai.agent.confidence", confidence)
		agentSpan.SetData("gen_ai.agent.iterations", result.Iterations)
		agentSpan.SetData("gen_ai.agent.escalated", result.Escalated)
		if len(result.ModelPath) > 0 {
			agentSpan.SetData("gen_ai.agent.model_path", result.ModelPath)
		}
		// Set agent output
		if result.Output != nil {
			if output, ok := result.Output.(string); ok && len(output) < 500 {
				agentSpan.SetAgentOutput(output)
			}
		}
	}

	if err != nil {
		slog.ErrorContext(agentCtx, "AI agent failed", "err", err)
		failureTags := map[string]string{
			"task_type": string(task.Type),
			"strategy":  string(strategy),
		}
		if modelName != "" {
			failureTags["model"] = modelName
		}
		if orchErr, ok := err.(*OrchestratorError); ok {
			failureTags["error_type"] = string(orchErr.Type)
			failureTags["retryable"] = fmt.Sprintf("%v", orchErr.Retryable)
		}
		observability.CaptureFailure(agentCtx, err, observability.FailureOptions{
			Component: "orchestrator",
			Operation: "execute_task",
			Tags:      failureTags,
		})
	} else {
		slog.InfoContext(agentCtx, "AI agent completed",
			"agent_name", agentName,
			"confidence", confidence,
			"success", success,
			"duration_ms", duration.Milliseconds(),
		)
	}

	// Finish agent span
	agentSpan.End(err)

	// Log only errors (success is tracked via metrics)
	if err != nil {
		slog.ErrorContext(ctx, "task failed", "task_type", task.Type, "duration", duration, "err", err)
	}

	return result, err
}

// IsConfigured returns true if at least one model is configured
func (o *Orchestrator) IsConfigured() bool {
	if len(o.models) == 0 {
		return false
	}
	// Check if default model has credentials
	defaultModel := o.models[RoleFast]
	if defaultModel == nil {
		return false
	}
	// Vertex AI doesn't require an API key (uses ADC)
	if defaultModel.Provider == ProviderGoogle && o.useVertex {
		return true
	}
	return o.getAPIKey(defaultModel.Provider) != ""
}

// SupportsVision returns true when a vision-capable model is available.
// Anthropic (Messages API) and Google (Gemini API / Vertex AI) support PDF and
// image vision; other OpenAI-compatible providers are not implemented.
func (o *Orchestrator) SupportsVision() bool {
	model := o.models[RoleVision]
	return model != nil && (model.Provider == ProviderAnthropic || model.Provider == ProviderGoogle)
}

// getAPIKey returns the API key for a provider
func (o *Orchestrator) getAPIKey(provider Provider) string {
	switch provider {
	case ProviderXAI:
		return o.cfg.XAIAPIKey
	case ProviderGoogle:
		return o.cfg.GoogleAPIKey
	case ProviderGroq:
		return o.cfg.GroqAPIKey
	case ProviderAnthropic:
		return o.cfg.AnthropicAPIKey
	default:
		return ""
	}
}

// getEndpoint returns the API endpoint for a provider
func (o *Orchestrator) getEndpoint(provider Provider, model string) string {
	if provider == ProviderGoogle && o.useVertex {
		// Vertex AI endpoint - handle global vs regional
		if o.vertexLocation == "global" {
			// Global endpoint doesn't have region prefix
			return fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/global/publishers/google/models/%s",
				o.vertexProject, model)
		}
		// Regional endpoint
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s",
			o.vertexLocation, o.vertexProject, o.vertexLocation, model)
	}
	return o.endpoints[provider]
}

// GetModel returns the model spec for a given role
func (o *Orchestrator) GetModel(role ModelRole) *ModelSpec {
	return o.models[role]
}

// GetVertexClient returns the Vertex AI genai client (if available)
func (o *Orchestrator) GetVertexClient() *genai.Client {
	return o.vertexClient
}

// CallPrompt sends a prompt directly to the LLM and returns the JSON response
// This bypasses the task system for simple prompt-based calls like pattern detection
func (o *Orchestrator) CallPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !o.IsConfigured() {
		return "", fmt.Errorf("orchestrator not configured")
	}

	model := o.models[RoleFast]
	if model == nil {
		return "", fmt.Errorf("no model configured")
	}

	messages := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := o.callModel(ctx, model, messages, false, nil, nil)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}
