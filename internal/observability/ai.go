package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// truncateSpanStr truncates s to maxLen bytes, appending "...[truncated]" if it was cut.
func truncateSpanStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "...[truncated]"
	}
	return s
}

// Span represents a traced operation with AI-specific metadata (OpenTelemetry → PostHog AI OTLP).
type Span struct {
	span      trace.Span
	ctx       context.Context
	startTime time.Time
}

func attrAny(k string, v any) attribute.KeyValue {
	switch x := v.(type) {
	case string:
		return attribute.String(k, x)
	case int:
		return attribute.Int(k, x)
	case int64:
		return attribute.Int64(k, x)
	case float64:
		return attribute.Float64(k, x)
	case bool:
		return attribute.Bool(k, x)
	default:
		return attribute.String(k, fmt.Sprint(x))
	}
}

// =============================================================================
// AI Request Spans - For LLM API calls
// =============================================================================

// AIRequestOptions configures an AI request span.
type AIRequestOptions struct {
	Model       string            // Required: model name (e.g., "gemini-2.0-flash")
	Provider    string            // Provider name (e.g., "google", "xai", "groq")
	Operation   string            // Operation type (e.g., "chat", "completion", "embedding")
	Messages    []any             // Input messages (will be JSON serialized)
	Tools       []any             // Available tools (will be JSON serialized)
	Temperature float64           // Model temperature
	MaxTokens   int               // Max tokens
	Tags        map[string]string // Additional tags
}

// StartAIRequest creates a span for an LLM API request (gen_ai.* naming for PostHog).
func StartAIRequest(ctx context.Context, opts AIRequestOptions) *Span {
	operation := opts.Operation
	if operation == "" {
		operation = "chat"
	}
	opName := fmt.Sprintf("gen_ai.%s", operation)

	hadParent := trace.SpanFromContext(ctx).SpanContext().IsValid()
	ctx, sp := Tracer().Start(ctx, opName, trace.WithSpanKind(trace.SpanKindClient))
	if !hadParent {
		sp.SetAttributes(attribute.String("trace_warning", "no_parent_span"))
	}

	sp.SetAttributes(
		attribute.String("gen_ai.request.model", opts.Model),
		attribute.String("gen_ai.operation.name", operation),
	)
	if opts.Provider != "" {
		sp.SetAttributes(
			attribute.String("gen_ai.system", opts.Provider),
			attribute.String("ai.provider", opts.Provider),
		)
	}
	if opts.Temperature > 0 {
		sp.SetAttributes(attribute.Float64("gen_ai.request.temperature", opts.Temperature))
	}
	if opts.MaxTokens > 0 {
		sp.SetAttributes(attribute.Int("gen_ai.request.max_tokens", opts.MaxTokens))
	}
	if len(opts.Messages) > 0 {
		if msgJSON, err := json.Marshal(opts.Messages); err == nil {
			sp.SetAttributes(attribute.String("gen_ai.request.messages", truncateSpanStr(string(msgJSON), 2000)))
		}
	}
	if len(opts.Tools) > 0 {
		if toolsJSON, err := json.Marshal(opts.Tools); err == nil {
			sp.SetAttributes(attribute.String("gen_ai.request.available_tools", truncateSpanStr(string(toolsJSON), 1000)))
		}
	}
	for k, v := range opts.Tags {
		sp.SetAttributes(attribute.String(k, v))
	}

	return &Span{span: sp, ctx: ctx, startTime: time.Now()}
}

// =============================================================================
// Agent Invocation Spans
// =============================================================================

// AgentOptions configures an agent invocation span.
type AgentOptions struct {
	Name        string            // Required: agent name
	Model       string            // Model being used
	Input       string            // Input to the agent
	Description string            // Optional description
	Tags        map[string]string // Additional tags
}

// StartAgent creates a span for an AI agent invocation.
func StartAgent(ctx context.Context, opts AgentOptions) *Span {
	spanName := "gen_ai.invoke_agent"
	ctx, sp := Tracer().Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindInternal))
	sp.SetAttributes(
		attribute.String("gen_ai.operation.name", "invoke_agent"),
		attribute.String("gen_ai.agent.name", opts.Name),
	)
	if opts.Model != "" {
		sp.SetAttributes(attribute.String("gen_ai.request.model", opts.Model))
	}
	if opts.Description != "" {
		sp.SetAttributes(attribute.String("gen_ai.agent.description", opts.Description))
	}
	if opts.Input != "" {
		sp.SetAttributes(attribute.String("gen_ai.agent.input", truncateSpanStr(opts.Input, 1000)))
	}
	for k, v := range opts.Tags {
		sp.SetAttributes(attribute.String(k, v))
	}
	return &Span{span: sp, ctx: ctx, startTime: time.Now()}
}

// =============================================================================
// Tool Execution Spans
// =============================================================================

// ToolOptions configures a tool execution span.
type ToolOptions struct {
	Name        string // Required: tool name
	Description string // Tool description
	Input       string // Input arguments (JSON)
	Type        string // Tool type: "function", "extension", "datastore"
}

// StartTool creates a span for a tool execution.
func StartTool(ctx context.Context, opts ToolOptions) *Span {
	ctx, sp := Tracer().Start(ctx, "gen_ai.execute_tool", trace.WithSpanKind(trace.SpanKindInternal))
	sp.SetAttributes(attribute.String("gen_ai.tool.name", opts.Name))
	if opts.Description != "" {
		sp.SetAttributes(attribute.String("gen_ai.tool.description", opts.Description))
	}
	if opts.Input != "" {
		sp.SetAttributes(attribute.String("gen_ai.tool.input", truncateSpanStr(opts.Input, 500)))
	}
	if opts.Type != "" {
		sp.SetAttributes(attribute.String("gen_ai.tool.type", opts.Type))
	}
	return &Span{span: sp, ctx: ctx, startTime: time.Now()}
}

// =============================================================================
// Pipeline / Workflow Spans
// =============================================================================

// PipelineOptions configures a pipeline span.
type PipelineOptions struct {
	Name        string
	Description string
	Steps       int
	Tags        map[string]string
}

// StartPipeline creates a span for an AI pipeline/workflow.
func StartPipeline(ctx context.Context, opts PipelineOptions) *Span {
	ctx, sp := Tracer().Start(ctx, "ai.pipeline", trace.WithSpanKind(trace.SpanKindInternal))
	sp.SetAttributes(attribute.String("ai.pipeline.name", opts.Name))
	if opts.Description != "" {
		sp.SetAttributes(attribute.String("ai.pipeline.description", opts.Description))
	}
	if opts.Steps > 0 {
		sp.SetAttributes(attribute.Int("ai.pipeline.steps", opts.Steps))
	}
	for k, v := range opts.Tags {
		sp.SetAttributes(attribute.String(k, v))
	}
	return &Span{span: sp, ctx: ctx, startTime: time.Now()}
}

// =============================================================================
// Background transaction (worker, etc.)
// =============================================================================

// StartTransaction starts a root span for background work (name must be ai.* for PostHog).
func StartTransaction(ctx context.Context, name, op string) (*Span, context.Context) {
	ctx, sp := Tracer().Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	sp.SetAttributes(attribute.String("ai.transaction.op", op))
	s := &Span{span: sp, ctx: ctx, startTime: time.Now()}
	return s, ctx
}

// =============================================================================
// Span Methods
// =============================================================================

// Context returns the context with this span attached.
func (s *Span) Context() context.Context {
	if s == nil || s.span == nil {
		return context.Background()
	}
	return s.ctx
}

// SetTokenUsage records token usage for the AI request.
func (s *Span) SetTokenUsage(inputTokens, outputTokens int) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", inputTokens),
		attribute.Int("gen_ai.usage.output_tokens", outputTokens),
		attribute.Int("gen_ai.usage.total_tokens", inputTokens+outputTokens),
	)
}

// SetTokenUsageDetailed records detailed token usage including cached and reasoning tokens.
func (s *Span) SetTokenUsageDetailed(inputTokens, outputTokens, cachedTokens, reasoningTokens int) {
	if s == nil || s.span == nil {
		return
	}
	s.SetTokenUsage(inputTokens, outputTokens)
	if cachedTokens > 0 {
		s.span.SetAttributes(attribute.Int("gen_ai.usage.input_tokens.cached", cachedTokens))
	}
	if reasoningTokens > 0 {
		s.span.SetAttributes(attribute.Int("gen_ai.usage.output_tokens.reasoning", reasoningTokens))
	}
}

// SetCost records the cost of the AI request (USD).
func (s *Span) SetCost(promptCost, completionCost float64) {
	if s == nil || s.span == nil {
		return
	}
	totalCost := promptCost + completionCost
	s.span.SetAttributes(
		attribute.Float64("gen_ai.usage.cost.total", totalCost),
		attribute.Float64("gen_ai.usage.cost.prompt", promptCost),
		attribute.Float64("gen_ai.usage.cost.completion", completionCost),
	)
}

// SetCostDetailed records detailed cost breakdown.
func (s *Span) SetCostDetailed(promptCost, completionCost, reasoningCost, cacheReadCost, cacheWriteCost float64) {
	if s == nil || s.span == nil {
		return
	}
	totalCost := promptCost + completionCost + reasoningCost + cacheReadCost + cacheWriteCost
	s.span.SetAttributes(
		attribute.Float64("gen_ai.usage.cost.total", totalCost),
		attribute.Float64("gen_ai.usage.cost.prompt", promptCost),
		attribute.Float64("gen_ai.usage.cost.completion", completionCost),
	)
	if reasoningCost > 0 {
		s.span.SetAttributes(attribute.Float64("gen_ai.usage.cost.reasoning", reasoningCost))
	}
	if cacheReadCost > 0 {
		s.span.SetAttributes(attribute.Float64("gen_ai.usage.cost.cache_read", cacheReadCost))
	}
	if cacheWriteCost > 0 {
		s.span.SetAttributes(attribute.Float64("gen_ai.usage.cost.cache_write", cacheWriteCost))
	}
}

// SetResponse records the response from the AI model.
func (s *Span) SetResponse(response string) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attribute.String("gen_ai.response.text", fmt.Sprintf(`[%q]`, truncateSpanStr(response, 1000))))
}

// SetToolCalls records tool calls made by the model.
func (s *Span) SetToolCalls(calls []any) {
	if s == nil || s.span == nil || len(calls) == 0 {
		return
	}
	if callsJSON, err := json.Marshal(calls); err == nil {
		s.span.SetAttributes(attribute.String("gen_ai.response.tool_calls", truncateSpanStr(string(callsJSON), 1000)))
	}
}

// SetToolOutput records the output from a tool execution.
func (s *Span) SetToolOutput(output string) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attribute.String("gen_ai.tool.output", truncateSpanStr(output, 500)))
}

// SetAgentOutput records the output from an agent execution.
func (s *Span) SetAgentOutput(output string) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attribute.String("gen_ai.agent.output", truncateSpanStr(output, 1000)))
}

// SetData sets arbitrary data on the span.
func (s *Span) SetData(key string, value any) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attrAny(key, value))
}

// SetTag sets a tag on the span.
func (s *Span) SetTag(key, value string) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetAttributes(attribute.String(key, value))
}

// SetError marks the span as failed with an error.
func (s *Span) SetError(err error) {
	if s == nil || s.span == nil || err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// SetStatus sets the span status code.
func (s *Span) SetStatus(st codes.Code) {
	if s == nil || s.span == nil {
		return
	}
	s.span.SetStatus(st, "")
}

// End finishes the span, optionally recording an error.
func (s *Span) End(err error) {
	if s == nil || s.span == nil {
		return
	}
	duration := time.Since(s.startTime)
	s.span.SetAttributes(attribute.Int64("duration_ms", duration.Milliseconds()))
	if err != nil {
		s.SetError(err)
	} else {
		s.span.SetStatus(codes.Ok, "")
	}
	s.span.End()
}
