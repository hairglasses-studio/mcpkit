package ralph

import (
	"fmt"
	"strings"
)

// Message represents a conversation message for context pruning.
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolArgs   string `json:"tool_args,omitempty"`
	Summarized bool   `json:"summarized,omitempty"`
}

// PruneStats tracks the token reduction achieved by a prune pass.
type PruneStats struct {
	OriginalTokens  int     `json:"original_tokens"`
	PrunedTokens    int     `json:"pruned_tokens"`
	Reduction       float64 `json:"reduction"`
	MessagesDropped int     `json:"messages_dropped"`
	ToolDedups      int     `json:"tool_dedups"`
	Truncations     int     `json:"truncations"`
}

// ContextPruner reduces conversation context to fit within a token budget.
type ContextPruner struct {
	MaxTokens       int
	TruncateLines   int
	SummaryMaxWords int
}

// NewContextPruner creates a pruner with the given token budget.
func NewContextPruner(maxTokens int) *ContextPruner {
	if maxTokens <= 0 {
		maxTokens = 100_000
	}
	return &ContextPruner{
		MaxTokens:       maxTokens,
		TruncateLines:   20,
		SummaryMaxWords: 30,
	}
}

// EstimateTokens returns a fast word-based token count approximation.
func EstimateTokens(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.3)
}

// Prune reduces messages to fit within MaxTokens.
func (p *ContextPruner) Prune(messages []Message) ([]Message, PruneStats) {
	if len(messages) == 0 {
		return messages, PruneStats{}
	}

	originalTokens := p.totalTokens(messages)
	stats := PruneStats{OriginalTokens: originalTokens}

	work := make([]Message, len(messages))
	copy(work, messages)

	work, stats.ToolDedups = p.dedupToolCalls(work)

	if p.totalTokens(work) <= p.MaxTokens {
		stats.PrunedTokens = p.totalTokens(work)
		stats.MessagesDropped = len(messages) - len(work)
		p.setReduction(&stats)
		return work, stats
	}

	work, stats.Truncations = p.truncateLongOutputs(work)

	if p.totalTokens(work) <= p.MaxTokens {
		stats.PrunedTokens = p.totalTokens(work)
		stats.MessagesDropped = len(messages) - len(work)
		p.setReduction(&stats)
		return work, stats
	}

	work, summarized := p.summarizeMiddle(work)
	stats.MessagesDropped += summarized

	stats.PrunedTokens = p.totalTokens(work)
	p.setReduction(&stats)
	return work, stats
}

func (p *ContextPruner) dedupToolCalls(msgs []Message) ([]Message, int) {
	if len(msgs) < 2 {
		return msgs, 0
	}

	type toolKey struct {
		name string
		args string
	}

	seen := make(map[toolKey]int)
	keep := make([]bool, len(msgs))
	dedups := 0

	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "tool" || m.ToolName == "" {
			keep[i] = true
			continue
		}
		k := toolKey{name: m.ToolName, args: m.ToolArgs}
		if _, exists := seen[k]; exists {
			dedups++
			continue
		}
		seen[k] = i
		keep[i] = true
	}

	out := make([]Message, 0, len(msgs)-dedups)
	for i, m := range msgs {
		if keep[i] {
			out = append(out, m)
		}
	}
	return out, dedups
}

func (p *ContextPruner) truncateLongOutputs(msgs []Message) ([]Message, int) {
	truncateLines := p.TruncateLines
	if truncateLines <= 0 {
		truncateLines = 20
	}
	threshold := truncateLines * 3

	truncations := 0
	for i := range msgs {
		if msgs[i].Role != "tool" {
			continue
		}
		lines := strings.Split(msgs[i].Content, "\n")
		if len(lines) <= threshold {
			continue
		}

		head := lines[:truncateLines]
		tail := lines[len(lines)-truncateLines:]
		omitted := len(lines) - 2*truncateLines
		msgs[i].Content = strings.Join(head, "\n") +
			fmt.Sprintf("\n\n[... %d lines truncated ...]\n\n", omitted) +
			strings.Join(tail, "\n")
		truncations++
	}
	return msgs, truncations
}

func (p *ContextPruner) summarizeMiddle(msgs []Message) ([]Message, int) {
	if len(msgs) <= 4 {
		return msgs, 0
	}

	protected := p.protectedIndices(msgs)

	summarized := 0
	for i := range msgs {
		if protected[i] {
			continue
		}
		if msgs[i].Summarized {
			continue
		}
		if p.totalTokens(msgs) <= p.MaxTokens {
			break
		}
		msgs[i].Content = p.summarizeContent(msgs[i].Content)
		msgs[i].Summarized = true
		summarized++
	}

	if p.totalTokens(msgs) > p.MaxTokens {
		var kept []Message
		for i, m := range msgs {
			if protected[i] || p.totalTokens(kept) < p.MaxTokens {
				kept = append(kept, m)
			} else {
				summarized++
			}
		}
		msgs = kept
	}

	return msgs, summarized
}

func (p *ContextPruner) protectedIndices(msgs []Message) map[int]bool {
	protected := make(map[int]bool)
	for i, m := range msgs {
		if m.Role == "system" {
			protected[i] = true
		}
	}
	userCount := 0
	for i := len(msgs) - 1; i >= 0 && userCount < 3; i-- {
		if msgs[i].Role == "user" {
			protected[i] = true
			userCount++
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			protected[i] = true
			break
		}
	}
	return protected
}

func (p *ContextPruner) summarizeContent(content string) string {
	maxWords := p.SummaryMaxWords
	if maxWords <= 0 {
		maxWords = 30
	}
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return content
	}
	var keyParts []string
	decisionMarkers := []string{"decided", "chose", "selected", "will use", "implemented", "created", "fixed", "changed"}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			keyParts = append(keyParts, line)
			break
		}
	}
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		for _, marker := range decisionMarkers {
			if strings.Contains(lower, marker) {
				keyParts = append(keyParts, strings.TrimSpace(line))
				break
			}
		}
		if len(keyParts) >= 3 {
			break
		}
	}
	summary := strings.Join(keyParts, " | ")
	words := strings.Fields(summary)
	if len(words) > maxWords {
		summary = strings.Join(words[:maxWords], " ") + "..."
	}
	return "[summary] " + summary
}

func (p *ContextPruner) totalTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokens(m.Content)
	}
	return total
}

func (p *ContextPruner) setReduction(stats *PruneStats) {
	if stats.OriginalTokens > 0 {
		stats.Reduction = 1.0 - float64(stats.PrunedTokens)/float64(stats.OriginalTokens)
	}
}
