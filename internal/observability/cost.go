package observability

// ModelPricing defines the cost per 1M tokens for input and output
// Prices are in USD per million tokens
// Source: Provider pricing pages (as of 2025)
type ModelPricing struct {
	InputCostPerMillion  float64 // Cost per 1M input tokens
	OutputCostPerMillion float64 // Cost per 1M output tokens
	// Optional: reasoning tokens (for models like o3)
	ReasoningCostPerMillion float64 // Cost per 1M reasoning tokens (0 if not applicable)
}

// GetModelPricing returns the pricing for a given model.
// Format: "provider/model" (e.g., "google/gemini-2.5-flash", "xai/grok-4-1")
func GetModelPricing(model string) ModelPricing {
	// Google models (Gemini)
	switch model {
	case "google/gemini-2.5-flash", "google/gemini-2.5-flash-lite":
		return ModelPricing{
			InputCostPerMillion:  0.075,  // $0.075 per 1M input tokens
			OutputCostPerMillion: 0.30,   // $0.30 per 1M output tokens
		}
	case "google/gemini-2.5-pro":
		return ModelPricing{
			InputCostPerMillion:  1.25,   // $1.25 per 1M input tokens
			OutputCostPerMillion: 5.00,   // $5.00 per 1M output tokens
		}
	case "google/gemini-3-flash-preview":
		return ModelPricing{
			InputCostPerMillion:  0.075,  // Same as 2.5-flash
			OutputCostPerMillion: 0.30,
		}
	case "google/gemini-3-pro-preview":
		return ModelPricing{
			InputCostPerMillion:  1.25,
			OutputCostPerMillion: 5.00,
		}
	case "google/gemma-3-1b-it":
		return ModelPricing{
			InputCostPerMillion:  0.05,   // Very cheap
			OutputCostPerMillion: 0.15,
		}
	// xAI models (Grok)
	case "xai/grok-4-1", "xai/grok-4-1-fast-reasoning-latest":
		return ModelPricing{
			InputCostPerMillion:  0.10,   // $0.10 per 1M input tokens
			OutputCostPerMillion: 0.30,   // $0.30 per 1M output tokens
		}
	// Groq models
	case "groq/llama-3.3-70b-versatile":
		return ModelPricing{
			InputCostPerMillion:  0.00,   // Free tier
			OutputCostPerMillion: 0.00,
		}
	// Anthropic models (Claude). Prices per 1M tokens, Anthropic public pricing.
	case "anthropic/claude-opus-4-8", "anthropic/claude-opus-4-1":
		return ModelPricing{
			InputCostPerMillion:  15.00,  // $15 per 1M input tokens
			OutputCostPerMillion: 75.00,  // $75 per 1M output tokens
		}
	case "anthropic/claude-sonnet-4-6", "anthropic/claude-sonnet-4-0":
		return ModelPricing{
			InputCostPerMillion:  3.00,   // $3 per 1M input tokens
			OutputCostPerMillion: 15.00,  // $15 per 1M output tokens
		}
	case "anthropic/claude-haiku-4-5-20251001", "anthropic/claude-3-5-haiku-latest":
		return ModelPricing{
			InputCostPerMillion:  1.00,   // $1 per 1M input tokens
			OutputCostPerMillion: 5.00,   // $5 per 1M output tokens
		}
	// Default: assume Google pricing (most common)
	default:
		// If it's a Google model we don't recognize, use flash pricing as default
		if len(model) > 7 && model[:7] == "google/" {
			return ModelPricing{
				InputCostPerMillion:  0.075,
				OutputCostPerMillion: 0.30,
			}
		}
		// Unrecognized Anthropic model — fall back to Sonnet pricing.
		if len(model) > 10 && model[:10] == "anthropic/" {
			return ModelPricing{
				InputCostPerMillion:  3.00,
				OutputCostPerMillion: 15.00,
			}
		}
		// Unknown model - return zero cost (will show as $0.00)
		return ModelPricing{
			InputCostPerMillion:  0.0,
			OutputCostPerMillion: 0.0,
		}
	}
}

// CalculateCost computes the cost in USD for token usage.
// Returns: promptCost, completionCost, reasoningCost
func CalculateCost(model string, inputTokens, outputTokens, reasoningTokens int) (float64, float64, float64) {
	pricing := GetModelPricing(model)
	
	// Calculate costs (tokens / 1,000,000 * cost per million)
	promptCost := float64(inputTokens) / 1_000_000.0 * pricing.InputCostPerMillion
	completionCost := float64(outputTokens) / 1_000_000.0 * pricing.OutputCostPerMillion
	
	var reasoningCost float64
	if reasoningTokens > 0 && pricing.ReasoningCostPerMillion > 0 {
		reasoningCost = float64(reasoningTokens) / 1_000_000.0 * pricing.ReasoningCostPerMillion
	}
	
	return promptCost, completionCost, reasoningCost
}
