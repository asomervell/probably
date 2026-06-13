package mcp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
)

// AuditLogger logs all MCP operations for security and compliance
type AuditLogger struct{}

// NewAuditLogger creates a new audit logger
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

// LogToolDiscovery logs tool discovery requests
func (a *AuditLogger) LogToolDiscovery(ctx context.Context, userID, ledgerID uuid.UUID) {
	slog.InfoContext(ctx, "mcp.audit: tool_discovery", "user_id", userID, "ledger_id", ledgerID)
}

// LogToolCall logs tool invocation requests
func (a *AuditLogger) LogToolCall(ctx context.Context, userID, ledgerID uuid.UUID, toolName string, arguments json.RawMessage) {
	redactedArgs := a.redactSensitiveData(arguments)
	slog.InfoContext(ctx, "mcp.audit: tool_call", "user_id", userID, "ledger_id", ledgerID, "tool", toolName, "args", string(redactedArgs))
}

// LogToolSuccess logs successful tool execution
func (a *AuditLogger) LogToolSuccess(ctx context.Context, userID, ledgerID uuid.UUID, toolName string) {
	slog.InfoContext(ctx, "mcp.audit: tool_success", "user_id", userID, "ledger_id", ledgerID, "tool", toolName)
}

// LogToolError logs tool execution errors
func (a *AuditLogger) LogToolError(ctx context.Context, userID, ledgerID uuid.UUID, toolName string, err error) {
	slog.ErrorContext(ctx, "mcp.audit: tool_error", "user_id", userID, "ledger_id", ledgerID, "tool", toolName, "err", err)
}

var sensitiveFields = map[string]bool{
	"password": true, "secret": true, "token": true,
	"api_key": true, "apiKey": true, "credit_card": true,
	"ssn": true, "social_security": true,
}

// redactSensitiveData removes or masks sensitive information from JSON
func (a *AuditLogger) redactSensitiveData(data json.RawMessage) json.RawMessage {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	redacted := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		if sensitiveFields[k] {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	result, err := json.Marshal(redacted)
	if err != nil {
		return data
	}
	return result
}
