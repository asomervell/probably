package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/asomervell/probably/internal/config"
)

// GenerateChatTitle generates a short LLM-derived title from a conversation's first message.
// Returns a best-effort title; falls back to the first few words of the message on any failure.
func GenerateChatTitle(ctx context.Context, cfg *config.Config, firstMessage string) string {
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		slog.WarnContext(ctx, "title generation: orchestrator init failed", "err", err)
		return chatTitleFallback(firstMessage)
	}

	prompt := fmt.Sprintf(`Generate a short 3-5 word title for this chat conversation. The title should capture the main topic. Return ONLY the title, no quotes or extra text.

User's first message: %s`, firstMessage)

	task := &Task{
		Type:     TaskTypeChat,
		Strategy: StrategySimple,
		Input: &ChatInput{
			Messages: []interface{}{
				map[string]interface{}{"role": "user", "content": prompt},
			},
		},
	}

	result, err := orch.Execute(ctx, task)
	if err != nil {
		slog.WarnContext(ctx, "title generation: execute failed", "err", err)
		return chatTitleFallback(firstMessage)
	}

	var title string
	switch v := result.Output.(type) {
	case string:
		title = v
	case map[string]interface{}:
		if t, ok := v["content"].(string); ok {
			title = t
		} else if t, ok := v["text"].(string); ok {
			title = t
		}
	}

	title = strings.TrimSpace(strings.Trim(title, `"'`))
	if len(title) > 100 {
		title = title[:100]
	}
	if title == "" {
		return chatTitleFallback(firstMessage)
	}
	return title
}

func chatTitleFallback(msg string) string {
	words := strings.Fields(msg)
	if len(words) > 5 {
		words = words[:5]
	}
	title := strings.Join(words, " ")
	if len(title) > 50 {
		title = title[:50] + "..."
	}
	return title
}
