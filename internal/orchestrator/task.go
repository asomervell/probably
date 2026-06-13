package orchestrator

import (
	"context"

	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/processing"
)

// TaskType defines the type of task being executed
type TaskType string

const (
	TaskTypeCategorize    TaskType = "categorize"
	TaskTypeCategorizeP2P TaskType = "categorize_p2p"
	TaskTypeChat          TaskType = "chat"
	TaskTypeChatSQL       TaskType = "chat_sql"   // For SQL generation
	TaskTypeChatTools     TaskType = "chat_tools" // Chat with tool calling (V2)
)

// Strategy defines the execution strategy
type Strategy string

const (
	StrategySimple     Strategy = "simple"     // Single model call (current behavior)
	StrategyEscalate   Strategy = "escalate"   // Cheap model first, escalate on low confidence
	StrategyConsensus  Strategy = "consensus"  // Parallel model calls with voting
	StrategySupervisor Strategy = "supervisor" // Planner + worker pattern
	StrategyVerify     Strategy = "verify"     // Generate -> check -> refine loop
)

// Task represents a task to be executed by the orchestrator
type Task struct {
	ID       string
	Type     TaskType
	Strategy Strategy // Override default strategy
	Input    any      // Task-specific input (TransactionContext, ChatMessage, etc.)
	Context  *TaskContext
}

// TaskContext provides additional context for task execution
type TaskContext struct {
	LedgerID    string
	UserID      string
	Metadata    map[string]string
	Constraints *TaskConstraints
}

// TaskConstraints defines limits and requirements for task execution
type TaskConstraints struct {
	MaxTokens          int
	MaxIterations      int
	TimeoutSeconds     int
	RequiredConfidence float64
}

// Result represents the result of a task execution
type Result struct {
	Output     any                    // Task-specific output ([]LLMResult, ChatResponse, etc.)
	Confidence float64                // Overall confidence (0.0-1.0)
	ModelPath  []string               // Which models were used (e.g., ["fast", "reasoning"])
	Escalated  bool                   // Whether escalation occurred
	Iterations int                    // Number of iterations/rounds
	Metadata   map[string]interface{} // Additional metadata
	// Thoughts contains the model's reasoning process (Gemini 3+ only)
	// Human-readable text that can be shown to users to explain AI decisions
	Thoughts []string // e.g., ["Analyzing transaction description...", "This looks like a grocery purchase"]
}

// CategorizeInput represents input for categorization tasks
type CategorizeInput struct {
	Transactions    []interface{} // TransactionContext or P2PTransactionContext
	Tags            []interface{} // *models.Tag
	Rules           []interface{} // *models.CategorizationRule
	EntitySearcher  interface{}   // EntitySearcher (optional)
	PurchaseMatcher interface{}   // PurchaseMatcher (optional)
	MyLifeContext   string        // Formatted "My Life" context (people, work, assets)
}

// ChatInput represents input for chat tasks
type ChatInput struct {
	Messages        []interface{}   // ChatMessage
	Context         interface{}     // ConversationContext
	GenerateSQL     bool            // Whether to generate SQL for chat queries
	VoiceMode       bool            // Whether to optimize responses for voice (more concise)
	ThoughtCallback ThoughtCallback // Optional callback for streaming thoughts
}

// ChatToolInput represents input for tool-based chat tasks (V2)
type ChatToolInput struct {
	Messages        []interface{}    // ChatMessage format
	SystemPrompt    string           // System prompt (without tool descriptions)
	ToolExecutor    ToolExecutor     // Executor for tool calls
	Tools           []ToolDefinition // Available tools
	ThoughtCallback ThoughtCallback  // Optional callback for streaming thoughts
}

// ToolDefinition describes a tool for the LLM
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolExecutor executes tool calls and returns results
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, arguments string) (string, error)
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	ToolName string `json:"tool_name"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
}

// ThoughtCallback is called when a new thought is received during streaming
type ThoughtCallback func(thought string)

// parseCategorizeInput converts the interface{} slices in CategorizeInput to their concrete types.
// Extracted here because this identical conversion appears in every strategy's executeCategorize.
func parseCategorizeInput(input *CategorizeInput) (
	[]processing.TransactionContext,
	[]*models.Tag,
	[]*models.CategorizationRule,
	EntitySearcher,
	PurchaseMatcher,
) {
	transactions := make([]processing.TransactionContext, 0, len(input.Transactions))
	for _, t := range input.Transactions {
		if txn, ok := t.(processing.TransactionContext); ok {
			transactions = append(transactions, txn)
		}
	}

	tags := make([]*models.Tag, 0, len(input.Tags))
	for _, t := range input.Tags {
		if tag, ok := t.(*models.Tag); ok {
			tags = append(tags, tag)
		}
	}

	rules := make([]*models.CategorizationRule, 0, len(input.Rules))
	for _, r := range input.Rules {
		if rule, ok := r.(*models.CategorizationRule); ok {
			rules = append(rules, rule)
		}
	}

	var entitySearcher EntitySearcher
	var purchaseMatcher PurchaseMatcher
	if input.EntitySearcher != nil {
		if es, ok := input.EntitySearcher.(EntitySearcher); ok {
			entitySearcher = es
		}
	}
	if input.PurchaseMatcher != nil {
		if pm, ok := input.PurchaseMatcher.(PurchaseMatcher); ok {
			purchaseMatcher = pm
		}
	}

	return transactions, tags, rules, entitySearcher, purchaseMatcher
}
