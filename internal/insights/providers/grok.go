package providers

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// ProviderConfig defines configuration for an OpenAI-compatible LLM provider
type ProviderConfig struct {
	Name         string
	BaseURL      string
	DefaultModel string
}

// Pre-defined provider configurations
var (
	GroqConfig = ProviderConfig{
		Name:         "groq",
		BaseURL:      "https://api.groq.com/openai/v1",
		DefaultModel: "llama-3.3-70b-versatile",
	}
	GrokConfig = ProviderConfig{
		Name:         "grok",
		BaseURL:      "https://api.x.ai/v1",
		DefaultModel: "grok-3",
	}
	VertexConfig = ProviderConfig{
		Name:         "vertex",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai/",
		DefaultModel: "gemini-1.5-pro",
	}
	ClaudeConfig = ProviderConfig{
		Name:         "claude",
		BaseURL:      "https://api.anthropic.com/v1/",
		DefaultModel: "claude-3-5-sonnet-20241022",
	}
)

// Provider is a unified OpenAI-compatible LLM provider
type Provider struct {
	client *openai.Client
	model  string
	name   string
}

// NewProvider creates a new OpenAI-compatible provider
func NewProvider(apiKey, model string, cfg ProviderConfig) *Provider {
	if apiKey == "" {
		return &Provider{name: cfg.Name}
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = cfg.BaseURL

	if model == "" {
		model = cfg.DefaultModel
	}

	return &Provider{
		client: openai.NewClientWithConfig(config),
		model:  model,
		name:   cfg.Name,
	}
}

func (p *Provider) Name() string       { return p.name }
func (p *Provider) ModelName() string  { return p.model }
func (p *Provider) IsConfigured() bool { return p.client != nil }

func (p *Provider) GenerateReport(ctx context.Context, req ReportRequest) (*ReportResponse, error) {
	if !p.IsConfigured() {
		return nil, fmt.Errorf("%s provider not configured", p.name)
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: buildReportSystemPrompt()},
			{Role: openai.ChatMessageRoleUser, Content: buildReportUserPrompt(req)},
		},
		Temperature: 0.3,
		MaxTokens:   4000,
	})
	if err != nil {
		return nil, fmt.Errorf("%s API error: %w", p.name, err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from %s", p.name)
	}

	return parseReportResponse(resp.Choices[0].Message.Content)
}

func (p *Provider) AnalyzeTransaction(ctx context.Context, req TransactionRequest) (*TransactionInsight, error) {
	if !p.IsConfigured() {
		return nil, fmt.Errorf("%s provider not configured", p.name)
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: buildTransactionSystemPrompt()},
			{Role: openai.ChatMessageRoleUser, Content: buildTransactionUserPrompt(req)},
		},
		Temperature: 0.3,
		MaxTokens:   500,
	})
	if err != nil {
		return nil, fmt.Errorf("%s API error: %w", p.name, err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from %s", p.name)
	}

	return parseTransactionInsight(resp.Choices[0].Message.Content)
}

// Backward-compatible constructors

func NewGrokProvider(apiKey, model string) *Provider {
	return NewProvider(apiKey, model, GrokConfig)
}

func NewGroqProvider(apiKey, model string) *Provider {
	return NewProvider(apiKey, model, GroqConfig)
}

func NewVertexProvider(apiKey, model string) *Provider {
	return NewProvider(apiKey, model, VertexConfig)
}

func NewClaudeProvider(apiKey, model string) *Provider {
	return NewProvider(apiKey, model, ClaudeConfig)
}
