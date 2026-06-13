package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConversationContext stores conversation history for a user+ledger session
type ConversationContext struct {
	UserID    uuid.UUID
	LedgerID  uuid.UUID
	Messages  []ConversationMessage
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.RWMutex
}

// ConversationMessage represents a single message in the conversation
type ConversationMessage struct {
	Role      string       // "user" or "assistant"
	Content   string       // The message content
	Timestamp time.Time    // When the message was sent
	SQL       string       // SQL query (stored internally, never exposed)
	Results   *QueryResult // Cached results (optional)
}

// ContextManager manages conversation contexts in memory
type ContextManager struct {
	contexts map[string]*ConversationContext
	mu       sync.RWMutex
	maxMessages int // Maximum messages to keep in context
	ttl      time.Duration // Time to live for inactive contexts
}

// NewContextManager creates a new context manager
func NewContextManager(maxMessages int, ttl time.Duration) *ContextManager {
	if maxMessages <= 0 {
		maxMessages = 10 // Default: last 10 messages
	}
	if ttl <= 0 {
		ttl = 1 * time.Hour // Default: 1 hour
	}
	
	cm := &ContextManager{
		contexts:    make(map[string]*ConversationContext),
		maxMessages: maxMessages,
		ttl:         ttl,
	}
	
	// Start cleanup goroutine
	go cm.cleanup()
	
	return cm
}

// GetOrCreate gets an existing context or creates a new one
func (cm *ContextManager) GetOrCreate(userID, ledgerID uuid.UUID) *ConversationContext {
	key := cm.contextKey(userID, ledgerID)
	
	cm.mu.RLock()
	ctx, exists := cm.contexts[key]
	cm.mu.RUnlock()
	
	if exists {
		ctx.mu.RLock()
		updated := ctx.UpdatedAt
		ctx.mu.RUnlock()
		
		// Check if context is still valid (not expired)
		if time.Since(updated) < cm.ttl {
			return ctx
		}
		// Context expired, remove it
		cm.mu.Lock()
		delete(cm.contexts, key)
		cm.mu.Unlock()
	}
	
	// Create new context
	newCtx := &ConversationContext{
		UserID:    userID,
		LedgerID:  ledgerID,
		Messages:  make([]ConversationMessage, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	cm.mu.Lock()
	cm.contexts[key] = newCtx
	cm.mu.Unlock()
	
	return newCtx
}

// AddMessage adds a message to the conversation context
func (cm *ContextManager) AddMessage(ctx *ConversationContext, role, content, sql string, results *QueryResult) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	
	msg := ConversationMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		SQL:       sql,
		Results:   results,
	}
	
	ctx.Messages = append(ctx.Messages, msg)
	ctx.UpdatedAt = time.Now()
	
	// Limit context window to last maxMessages
	if len(ctx.Messages) > cm.maxMessages {
		// Keep the last maxMessages messages
		ctx.Messages = ctx.Messages[len(ctx.Messages)-cm.maxMessages:]
	}
}

// GetMessages returns a copy of the conversation messages
func (ctx *ConversationContext) GetMessages() []ConversationMessage {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	
	// Return a copy to prevent external modification
	messages := make([]ConversationMessage, len(ctx.Messages))
	copy(messages, ctx.Messages)
	return messages
}

// contextKey generates a unique key for a user+ledger combination
func (cm *ContextManager) contextKey(userID, ledgerID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", userID.String(), ledgerID.String())
}

// cleanup periodically removes expired contexts
func (cm *ContextManager) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		cm.mu.Lock()
		
		for key, ctx := range cm.contexts {
			ctx.mu.RLock()
			updated := ctx.UpdatedAt
			ctx.mu.RUnlock()
			
			if now.Sub(updated) > cm.ttl {
				delete(cm.contexts, key)
			}
		}
		
		cm.mu.Unlock()
	}
}
