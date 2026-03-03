package terminal

import (
	"context"
	"fmt"
	"strings"

	"github.com/matst80/go-ai-cli/pkg/ollama"
)

// ManageContext checks the conversation history token limit and summarizes if necessary.
// It returns true if summarization occurred, false otherwise.
func ManageContext(ctx context.Context, client *ollama.Client, req *ollama.ChatRequest) (bool, error) {
	if req == nil || len(req.Messages) <= 2 {
		return false, nil // Not enough messages to manage
	}

	numCtx := 16384 // Default
	if req.Options != nil {
		if val, ok := req.Options["num_ctx"].(int); ok {
			numCtx = val
		} else if val, ok := req.Options["num_ctx"].(float64); ok {
			numCtx = int(val)
		}
	}

	// Calculate approximate token count (chars / 4)
	totalChars := 0
	for _, msg := range req.Messages {
		totalChars += len(msg.Content)
		if msg.ReasoningContent != "" {
			totalChars += len(msg.ReasoningContent)
		}
		for _, toolCall := range msg.ToolCalls {
			totalChars += len(toolCall.Function.Arguments)
		}
	}

	approxTokens := totalChars / 4
	limit := int(float64(numCtx) * 0.8)

	// Trigger condition: exceeds 80% of num_ctx OR more than 10 messages
	needsSummarization := approxTokens > limit || len(req.Messages) > 10

	if !needsSummarization {
		return false, nil
	}

	// Find System message (usually index 0)
	sysIndex := 0
	var sysMessage ollama.Message
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		sysMessage = req.Messages[0]
		sysIndex = 1
	}

	// Calculate how many recent messages we can keep without exceeding ~50% of the limit
	// to leave room for the summary and new responses.
	recentMessages := []ollama.Message{}
	charsToKeep := 0
	keepTargetLimit := (numCtx / 2) * 4 // Characters

	// Walk backwards from the end to find recent messages to keep
	for i := len(req.Messages) - 1; i >= sysIndex; i-- {
		msg := req.Messages[i]
		msgChars := len(msg.Content)
		if msg.ReasoningContent != "" {
			msgChars += len(msg.ReasoningContent)
		}
		for _, tc := range msg.ToolCalls {
			msgChars += len(tc.Function.Arguments)
		}

		// Don't keep more than 4 recent messages, and ensure they fit within the budget
		if len(recentMessages) < 4 && charsToKeep+msgChars < keepTargetLimit {
			recentMessages = append([]ollama.Message{msg}, recentMessages...)
			charsToKeep += msgChars
		} else {
			break
		}
	}

	// The rest will be summarized
	summarizeEndIndex := len(req.Messages) - len(recentMessages)
	if summarizeEndIndex <= sysIndex {
		// Nothing to summarize (edge case where recent messages are huge)
		// Fallback: just keep system and drop the oldest
		req.Messages = append([]ollama.Message{sysMessage}, recentMessages...)
		return false, nil
	}

	messagesToSummarize := req.Messages[sysIndex:summarizeEndIndex]
	var textToSummarize strings.Builder
	for _, m := range messagesToSummarize {
		textToSummarize.WriteString(fmt.Sprintf("[%s]: %s\n", strings.ToUpper(m.Role), m.Content))
		for _, tc := range m.ToolCalls {
			textToSummarize.WriteString(fmt.Sprintf("[TOOL CALL %s]: %s\n", tc.Function.Name, string(tc.Function.Arguments)))
		}
	}

	summarizeReq := ollama.ChatRequest{
		Model: req.Model,
		Messages: []ollama.Message{
			{
				Role:    "system",
				Content: "Summarize the following conversation context succinctly, focusing on key facts, tool results, and user intent to be used as context for future interactions. Keep the summary short.",
			},
			{
				Role:    "user",
				Content: textToSummarize.String(),
			},
		},
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.3, // Lower temp for factual summary
			"num_ctx":     numCtx,
		},
	}

	// Blocking call for summarization
	summaryText, err := client.GenerateResponse(ctx, summarizeReq)
	if err != nil {
		return false, fmt.Errorf("summarization failed: %w", err)
	}

	// Reconstruct message history
	newMessages := []ollama.Message{sysMessage}
	newMessages = append(newMessages, ollama.Message{
		Role:    "system",
		Content: "Summary of previous conversation:\n" + summaryText,
	})
	newMessages = append(newMessages, recentMessages...)

	req.Messages = newMessages
	return true, nil
}
