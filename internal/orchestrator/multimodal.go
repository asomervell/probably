package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

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

	// Calling vision model

	if model.Provider != ProviderGoogle {
		return nil, fmt.Errorf("vision requires Google provider (Vertex AI or Gemini API); provider %q does not support vision", model.Provider)
	}

	if o.useVertex {
		return o.callVertexVision(ctx, model, req)
	}
	return o.callGeminiVision(ctx, model, req)
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

// mapVertexModelName maps model names to Vertex AI format if needed
func (o *Orchestrator) mapVertexModelName(model string) string {
	// Vertex AI model names are generally the same, but we can add mappings here if needed
	modelMap := map[string]string{
		// Add any model name mappings here if needed
	}

	if mapped, ok := modelMap[model]; ok {
		return mapped
	}

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
