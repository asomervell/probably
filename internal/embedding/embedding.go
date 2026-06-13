// Package embedding provides vector embedding generation for entities and patterns.
// Embeddings enable semantic similarity search for cold-start pattern detection
// and entity clustering.
package embedding

import (
	"context"
	"fmt"
	"strings"

	"github.com/asomervell/probably/internal/config"
	"google.golang.org/genai"
)

// EmbeddingModel constants
const (
	// ModelTextEmbedding004 is Google's latest text embedding model (768 dimensions)
	ModelTextEmbedding004 = "text-embedding-004"
	// ModelTextMultilingual002 supports 100+ languages (768 dimensions)
	ModelTextMultilingual002 = "text-multilingual-embedding-002"
	// DefaultEmbeddingModel is the default model to use
	DefaultEmbeddingModel = ModelTextEmbedding004
	// DefaultDimensions is the output dimension for text-embedding-004
	DefaultDimensions = 768
)

// Service provides embedding generation capabilities
type Service struct {
	client    *genai.Client
	model     string
	useVertex bool
}

// Config holds embedding service configuration
type Config struct {
	// UseVertex indicates whether to use Vertex AI or Google AI Studio
	UseVertex bool
	// VertexProject is the GCP project ID (required for Vertex)
	VertexProject string
	// VertexLocation is the GCP region (defaults to us-central1)
	VertexLocation string
	// GoogleAPIKey is the API key for Google AI Studio (required if not using Vertex)
	GoogleAPIKey string
	// Model is the embedding model to use (defaults to text-embedding-004)
	Model string
}

// NewService creates a new embedding service
func NewService(ctx context.Context, cfg *Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	model := cfg.Model
	if model == "" {
		model = DefaultEmbeddingModel
	}

	var client *genai.Client
	var err error

	if cfg.UseVertex {
		if cfg.VertexProject == "" {
			return nil, fmt.Errorf("vertex project is required when using Vertex AI")
		}
		location := cfg.VertexLocation
		if location == "" {
			location = "us-central1"
		}
		client, err = genai.NewClient(ctx, &genai.ClientConfig{
			Project:  cfg.VertexProject,
			Location: location,
			Backend:  genai.BackendVertexAI,
		})
	} else {
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("Google API key is required when not using Vertex AI")
		}
		client, err = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cfg.GoogleAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Service{
		client:    client,
		model:     model,
		useVertex: cfg.UseVertex,
	}, nil
}

// NewServiceFromConfig creates a new embedding service from the application config
func NewServiceFromConfig(ctx context.Context, cfg *config.Config) (*Service, error) {
	embCfg := &Config{
		UseVertex:      cfg.VertexProject != "",
		VertexProject:  cfg.VertexProject,
		VertexLocation: cfg.VertexLocation,
		GoogleAPIKey:   cfg.GoogleAPIKey,
		Model:          DefaultEmbeddingModel,
	}
	return NewService(ctx, embCfg)
}

// EmbedText generates an embedding for a single text string
func (s *Service) EmbedText(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedTexts generates embeddings for multiple text strings (batch)
func (s *Service) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Create content parts for each text
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}

	// Call the embedding API
	result, err := s.client.Models.EmbedContent(ctx, s.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("embedding API call failed: %w", err)
	}

	// Extract embeddings from result
	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Model returns the embedding model name
func (s *Service) Model() string {
	return s.model
}

// Close closes the embedding service
func (s *Service) Close() {
	// The genai client doesn't have a Close method
	// This is here for future compatibility
}

// EntityEmbeddingInput represents the data needed to generate an entity embedding
type EntityEmbeddingInput struct {
	Name        string
	Type        string
	Subtype     string
	Description string
	Website     string
}

// BuildEntityText creates a text representation of an entity for embedding
func BuildEntityText(input *EntityEmbeddingInput) string {
	var parts []string

	// Entity name is most important
	parts = append(parts, input.Name)

	// Add type context
	if input.Type != "" {
		parts = append(parts, fmt.Sprintf("Type: %s", input.Type))
	}
	if input.Subtype != "" {
		parts = append(parts, fmt.Sprintf("Category: %s", input.Subtype))
	}

	// Add description if available
	if input.Description != "" {
		// Truncate long descriptions
		desc := input.Description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}
		parts = append(parts, desc)
	}

	// Website domain can help with categorization
	if input.Website != "" {
		parts = append(parts, fmt.Sprintf("Website: %s", input.Website))
	}

	return strings.Join(parts, ". ")
}

// categorizeAmount converts cents to a human-readable category
func categorizeAmount(cents int64) string {
	dollars := float64(cents) / 100
	switch {
	case dollars < 5:
		return "under $5"
	case dollars < 15:
		return "$5-15"
	case dollars < 30:
		return "$15-30"
	case dollars < 50:
		return "$30-50"
	case dollars < 100:
		return "$50-100"
	case dollars < 200:
		return "$100-200"
	case dollars < 500:
		return "$200-500"
	default:
		return "over $500"
	}
}

// IsConfigured returns true if the embedding service is configured
func (s *Service) IsConfigured() bool {
	return s.client != nil
}

// TransactionEmbeddingInput represents the data needed to generate a transaction embedding
type TransactionEmbeddingInput struct {
	Description  string
	DisplayTitle string
	EntityName   string
	PatternType  string
	AmountCents  int64
}

// BuildTransactionText creates a text representation of a transaction for embedding
func BuildTransactionText(input *TransactionEmbeddingInput) string {
	var parts []string

	// Display title or description is most important
	if input.DisplayTitle != "" {
		parts = append(parts, input.DisplayTitle)
	} else if input.Description != "" {
		parts = append(parts, input.Description)
	}

	// Entity name provides merchant context
	if input.EntityName != "" {
		parts = append(parts, fmt.Sprintf("Merchant: %s", input.EntityName))
	}

	// Amount category
	if input.AmountCents != 0 {
		amountCategory := categorizeAmount(input.AmountCents)
		parts = append(parts, fmt.Sprintf("Amount: %s", amountCategory))
	}

	// Pattern type if known
	if input.PatternType != "" && input.PatternType != "none" {
		parts = append(parts, fmt.Sprintf("Type: %s", input.PatternType))
	}

	return strings.Join(parts, ". ")
}

