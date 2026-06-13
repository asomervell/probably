package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/genai"
)

// VisionRequest represents a request for vision/multimodal processing
type VisionRequest struct {
	Prompt   string // The extraction prompt
	Document []byte // Raw file bytes (PDF, image)
	MimeType string // application/pdf, image/jpeg, etc.
}

// VisionResponse represents the response from a vision model
type VisionResponse struct {
	Content    string // Raw JSON response from model
	TokensUsed int
	Model      string // Which model was used
}

// CallVision processes a document/image using a vision-capable model
// This is the core method that extraction service will use
func (o *Orchestrator) CallVision(ctx context.Context, req *VisionRequest) (*VisionResponse, error) {
	model := o.models[RoleVision]
	if model == nil {
		return nil, fmt.Errorf("no vision model configured")
	}

	// Route to the provider-specific vision implementation.
	switch model.Provider {
	case ProviderAnthropic:
		return o.callAnthropicVision(ctx, model, req)
	case ProviderGoogle:
		if o.useVertex {
			return o.callVertexVision(ctx, model, req)
		}
		return o.callGeminiVision(ctx, model, req)
	default:
		return nil, fmt.Errorf("provider %q does not support vision (supported: anthropic, google)", model.Provider)
	}
}

// callAnthropicVision uses the Anthropic Messages API for vision/document
// processing. It supports both images (image/*) and PDFs (application/pdf):
// PDFs are sent as "document" content blocks, images as "image" blocks.
func (o *Orchestrator) callAnthropicVision(ctx context.Context, model *ModelSpec, req *VisionRequest) (*VisionResponse, error) {
	apiKey := o.getAPIKey(ProviderAnthropic)
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for Anthropic vision")
	}

	// Build the document/image source block based on MIME type.
	encoded := base64.StdEncoding.EncodeToString(req.Document)
	var sourceBlock map[string]any
	switch {
	case req.MimeType == "application/pdf":
		sourceBlock = map[string]any{
			"type": "document",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "application/pdf",
				"data":       encoded,
			},
		}
	case strings.HasPrefix(req.MimeType, "image/"):
		sourceBlock = map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": req.MimeType,
				"data":       encoded,
			},
		}
	default:
		return nil, fmt.Errorf("unsupported vision MIME type for Anthropic: %q (supported: application/pdf, image/*)", req.MimeType)
	}

	reqBody := map[string]any{
		"model":      model.Model,
		"max_tokens": 16384,
		"system":     buildStatementExtractionSystemPrompt(),
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					sourceBlock,
					{"type": "text", "text": req.Prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := o.endpoints[ProviderAnthropic] + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Anthropic vision API error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic vision API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic vision response: %w", err)
	}

	if apiResp.StopReason == "max_tokens" {
		slog.WarnContext(ctx, "anthropic vision response truncated due to max tokens")
		return nil, fmt.Errorf("response was truncated due to token limit")
	}

	var sb strings.Builder
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	responseText := sb.String()
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Anthropic vision API")
	}

	return &VisionResponse{
		Content:    responseText,
		TokensUsed: apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		Model:      model.Model,
	}, nil
}

// callVertexVision uses Vertex AI GenAI SDK for vision processing
func (o *Orchestrator) callVertexVision(ctx context.Context, model *ModelSpec, req *VisionRequest) (*VisionResponse, error) {
	if o.vertexClient == nil {
		return nil, fmt.Errorf("Vertex AI client not initialized")
	}

	// Map model name for Vertex AI
	vertexModel := o.mapVertexModelName(model.Model)

	// Build content with text and document
	parts := []*genai.Part{
		genai.NewPartFromText(req.Prompt),
		genai.NewPartFromBytes(req.Document, req.MimeType),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	// Build system prompt for statement extraction
	systemPrompt := buildStatementExtractionSystemPrompt()

	// Configure generation
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleModel),
		Temperature:       genai.Ptr[float32](0.1),
		MaxOutputTokens:   16384, // Increased for larger statements
		ResponseMIMEType:  "application/json",
	}

	// Call the API
	apiResult, err := o.vertexClient.Models.GenerateContent(ctx, vertexModel, contents, config)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI vision API error: %w", err)
	}

	// Check for truncation or other issues
	if len(apiResult.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Vertex AI response")
	}

	candidate := apiResult.Candidates[0]
	if candidate.FinishReason == genai.FinishReasonMaxTokens {
		slog.WarnContext(ctx, "vertex AI response truncated due to max tokens")
		return nil, fmt.Errorf("response was truncated due to token limit")
	}

	// Extract text from response
	responseText := apiResult.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Vertex AI")
	}

	// Vision response received

	return &VisionResponse{
		Content:    responseText,
		TokensUsed: int(apiResult.UsageMetadata.TotalTokenCount),
		Model:      vertexModel,
	}, nil
}

// callGeminiVision uses Google AI Studio (Gemini API) for vision processing.
func (o *Orchestrator) callGeminiVision(ctx context.Context, model *ModelSpec, req *VisionRequest) (*VisionResponse, error) {
	if o.cfg == nil || o.cfg.GoogleAPIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is required for Google vision model when Vertex AI is not configured")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  o.cfg.GoogleAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini API client: %w", err)
	}

	parts := []*genai.Part{
		genai.NewPartFromText(req.Prompt),
		genai.NewPartFromBytes(req.Document, req.MimeType),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(buildStatementExtractionSystemPrompt(), genai.RoleModel),
		Temperature:       genai.Ptr[float32](0.1),
		MaxOutputTokens:   16384,
		ResponseMIMEType:  "application/json",
	}

	apiResult, err := client.Models.GenerateContent(ctx, model.Model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini vision API error: %w", err)
	}
	if len(apiResult.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini vision response")
	}

	candidate := apiResult.Candidates[0]
	if candidate.FinishReason == genai.FinishReasonMaxTokens {
		slog.WarnContext(ctx, "gemini vision response truncated due to max tokens")
		return nil, fmt.Errorf("response was truncated due to token limit")
	}

	responseText := apiResult.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Gemini vision API")
	}

	return &VisionResponse{
		Content:    responseText,
		TokensUsed: int(apiResult.UsageMetadata.TotalTokenCount),
		Model:      model.Model,
	}, nil
}

func (o *Orchestrator) mapVertexModelName(model string) string {
	return model
}

// buildStatementExtractionSystemPrompt returns the system prompt for statement extraction
func buildStatementExtractionSystemPrompt() string {
	return `You are a financial document extraction assistant. Your job is to extract all transactions from bank statements.

Extract every transaction row you can see. Be thorough and accurate.

Return ONLY valid JSON in this exact format:
{
  "transactions": [
    {
      "date": "YYYY-MM-DD",
      "description": "Transaction description",
      "amount_cents": 12345,
      "merchant": "Merchant name (optional)",
      "category": "Category (optional)",
      "confidence": 0.95
    }
  ]
}

Rules:
- Date format: YYYY-MM-DD (e.g., "2024-01-15")
- Amount in cents: positive for credits/deposits, negative for debits/withdrawals
- For asset accounts: positive = money coming in, negative = money going out
- For liability accounts: positive = charges/purchases, negative = payments/credits
- Extract ALL transactions you can see, even if some details are unclear
- Confidence: 0.0-1.0 based on how clear the transaction is in the document`
}
