package extraction

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/asomervell/probably/internal/storage"
)

// ExtractedTransaction represents a transaction extracted from a statement
type ExtractedTransaction struct {
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	AmountCents int64     `json:"amount_cents"` // Positive for credits, negative for debits
	Merchant    string    `json:"merchant,omitempty"`
	Category    string    `json:"category,omitempty"`
	Confidence  float64   `json:"confidence,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to handle date format "YYYY-MM-DD"
func (e *ExtractedTransaction) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with string date field
	var temp struct {
		Date        string  `json:"date"`
		Description string  `json:"description"`
		AmountCents int64   `json:"amount_cents"`
		Merchant    *string `json:"merchant,omitempty"` // Use pointer to handle null
		Category    string  `json:"category,omitempty"`
		Confidence  float64 `json:"confidence,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Parse the date string in "YYYY-MM-DD" format
	parsedDate, err := time.Parse("2006-01-02", temp.Date)
	if err != nil {
		return fmt.Errorf("invalid date format %q: %w", temp.Date, err)
	}

	e.Date = parsedDate
	e.Description = temp.Description
	e.AmountCents = temp.AmountCents
	if temp.Merchant != nil {
		e.Merchant = *temp.Merchant
	}
	e.Category = temp.Category
	e.Confidence = temp.Confidence

	return nil
}

// StatementExtractor extracts transactions from bank statements using the orchestrator
type StatementExtractor struct {
	cfg          *config.Config
	storage      storage.Storage
	orchestrator *orchestrator.Orchestrator
}

// NewStatementExtractor creates a new statement extractor using orchestrator
func NewStatementExtractor(cfg *config.Config, storage storage.Storage, orch *orchestrator.Orchestrator) (*StatementExtractor, error) {
	if orch == nil {
		return nil, fmt.Errorf("orchestrator is required")
	}

	if !orch.SupportsVision() {
		return nil, fmt.Errorf("orchestrator does not support vision (no vision model configured)")
	}

	return &StatementExtractor{
		cfg:          cfg,
		storage:      storage,
		orchestrator: orch,
	}, nil
}

// ExtractTransactions extracts transactions from a statement file stored in GCS
func (e *StatementExtractor) ExtractTransactions(ctx context.Context, gcsPath string, accountType models.AccountType) ([]ExtractedTransaction, error) {
	return e.extractWithOrchestrator(ctx, gcsPath, accountType)
}

// extractWithOrchestrator uses the orchestrator's CallVision method
func (e *StatementExtractor) extractWithOrchestrator(ctx context.Context, gcsPath string, accountType models.AccountType) ([]ExtractedTransaction, error) {
	// Read the file from storage
	fileData, contentType, err := e.readFileFromStorage(ctx, gcsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read statement file: %w", err)
	}

	// Build the user prompt
	userPrompt := fmt.Sprintf(`Extract all transactions from this %s statement.

Account type: %s
- For asset accounts: positive amounts = money coming in, negative = money going out
- For liability accounts: positive amounts = charges/purchases, negative = payments/credits

Extract every transaction row you can see. Be thorough and accurate.`,
		contentType, accountType)

	// Use orchestrator's CallVision method
	visionResp, err := e.orchestrator.CallVision(ctx, &orchestrator.VisionRequest{
		Prompt:   userPrompt,
		Document: fileData,
		MimeType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call vision model: %w", err)
	}

	responseText := visionResp.Content

	var parsedResult struct {
		Transactions []ExtractedTransaction `json:"transactions"`
	}
	if err := json.Unmarshal([]byte(responseText), &parsedResult); err != nil {
		// Try to extract JSON from markdown code blocks
		cleaned := strings.TrimSpace(responseText)
		if strings.Contains(cleaned, "```") {
			start := strings.Index(cleaned, "```json")
			if start == -1 {
				start = strings.Index(cleaned, "```")
			}
			if start != -1 {
				// Find the end of the opening backticks (either "```json" or "```")
				backtickEnd := start
				if strings.HasPrefix(cleaned[start:], "```json") {
					backtickEnd = start + len("```json")
				} else if strings.HasPrefix(cleaned[start:], "```") {
					backtickEnd = start + len("```")
				}

				// Try to find newline after backticks
				newlinePos := strings.Index(cleaned[backtickEnd:], "\n")
				if newlinePos != -1 {
					jsonStart := backtickEnd + newlinePos + 1
					end := strings.Index(cleaned[jsonStart:], "```")
					if end != -1 {
						cleaned = strings.TrimSpace(cleaned[jsonStart : jsonStart+end])
					} else {
						cleaned = strings.TrimSpace(cleaned[jsonStart:])
					}
				} else {
					// No newline found, start right after the backticks
					jsonStart := backtickEnd
					end := strings.Index(cleaned[jsonStart:], "```")
					if end != -1 {
						cleaned = strings.TrimSpace(cleaned[jsonStart : jsonStart+end])
					} else {
						cleaned = strings.TrimSpace(cleaned[jsonStart:])
					}
				}
			}
		}
		if err := json.Unmarshal([]byte(cleaned), &parsedResult); err != nil {
			// Check if the JSON is incomplete (truncated)
			if strings.Contains(err.Error(), "unexpected end of JSON input") {
				// Try to find the last complete transaction
				slog.InfoContext(ctx, "JSON appears truncated, attempting to recover partial data")
				// Find the last complete transaction by looking for closing braces
				lastCompleteIdx := strings.LastIndex(cleaned, "}")
				if lastCompleteIdx > 0 {
					// Try to find the last complete transaction object
					// Look backwards for a complete transaction entry
					transStart := strings.LastIndex(cleaned[:lastCompleteIdx+1], `"transactions": [`)
					if transStart != -1 {
						// Try to extract up to the last complete transaction
						partialJSON := cleaned[:lastCompleteIdx+1] + "\n  ]\n}"
						if err := json.Unmarshal([]byte(partialJSON), &parsedResult); err == nil {
							slog.InfoContext(ctx, "recovered transactions from truncated response", "count", len(parsedResult.Transactions))
							return parsedResult.Transactions, nil
						}
					}
				}
				return nil, fmt.Errorf("response was truncated and could not be recovered. The JSON was cut off mid-transaction. "+
					"This may indicate the statement has too many transactions. "+
					"Try splitting the statement or increasing MaxOutputTokens. "+
					"Error: %w (response length: %d chars, last 200 chars: %s)",
					err, len(responseText), getLastChars(responseText, 200))
			}
			return nil, fmt.Errorf("failed to parse extraction response: %w (response length: %d chars, first 500 chars: %s)",
				err, len(responseText), getFirstChars(responseText, 500))
		}
	}

	slog.InfoContext(ctx, "extracted transactions from statement", "count", len(parsedResult.Transactions))
	return parsedResult.Transactions, nil
}

// readFileFromStorage reads a file from storage
func (e *StatementExtractor) readFileFromStorage(ctx context.Context, gcsPath string) ([]byte, string, error) {
	data, err := e.storage.Read(ctx, gcsPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file from storage (path: %s): %w", gcsPath, err)
	}

	// Determine content type from file extension or use default
	contentType := "application/pdf"
	if strings.HasSuffix(strings.ToLower(gcsPath), ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(strings.ToLower(gcsPath), ".jpg") || strings.HasSuffix(strings.ToLower(gcsPath), ".jpeg") {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}

// getLastChars returns the last n characters of a string
func getLastChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// getFirstChars returns the first n characters of a string
func getFirstChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
