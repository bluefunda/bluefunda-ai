package cmd

import "github.com/bluefunda/bluefunda-ai/internal/tools"

// Trimming history must keep it a valid request: every request needs the
// user message that started the current turn, and every tool result must
// follow the assistant message whose tool call it answers. Dropping messages
// blindly from the front breaks both in a long tool-calling turn, and the
// provider then rejects the request with an error that looks like a context
// overflow.

// trimmedToolChars is the size tool results are cut to when history still
// doesn't fit after dropping whole turns and older tool rounds.
const trimmedToolChars = 2_000

// recentTail returns what compaction keeps after the summary: the last user
// message, then the last n messages, starting at an assistant message so no
// tool result loses its call. Large tool results are capped.
func recentTail(history []codeMessage, n int) []codeMessage {
	u := lastUserIndex(history)
	if u < 0 {
		return nil
	}
	start := max(len(history)-n, u+1)
	for start < len(history) && history[start].Role != "assistant" {
		start++
	}
	tail := append([]codeMessage{history[u]}, history[start:]...)
	return capToolResults(tail, tools.MaxOutputChars)
}

// dropOldestTurns shrinks history until estimateTokens is below targetTokens,
// keeping it valid. In order, it:
//  1. drops whole earlier turns (each from a user message to the next);
//  2. drops the oldest assistant/tool rounds of the current turn, always
//     keeping its user message and the latest round;
//  3. cuts the remaining tool results down to trimmedToolChars.
//
// System messages are always kept.
func dropOldestTurns(history []codeMessage, targetTokens int) []codeMessage {
	var sys, convo []codeMessage
	for _, m := range history {
		if m.Role == "system" {
			sys = append(sys, m)
		} else {
			convo = append(convo, m)
		}
	}
	fits := func() bool {
		return estimateTokens(append(append([]codeMessage(nil), sys...), convo...)) <= targetTokens
	}

	// 1. Whole earlier turns.
	for !fits() {
		next := -1
		for i := 1; i < len(convo); i++ {
			if convo[i].Role == "user" {
				next = i
				break
			}
		}
		if next < 0 {
			break // only the current turn is left
		}
		convo = convo[next:]
	}
	// Whatever precedes the first user message can't be sent on its own.
	if u := firstUserIndex(convo); u > 0 {
		convo = convo[u:]
	}

	// 2. Oldest rounds of the current turn. A round starts at an assistant
	// message and runs up to the next one.
	for !fits() && len(convo) > 0 && convo[0].Role == "user" {
		first := nextAssistant(convo, 1)
		second := nextAssistant(convo, first+1)
		if first < 0 || second < 0 {
			break // keep the latest round
		}
		convo = append(convo[:1:1], convo[second:]...)
	}

	// 3. Cut tool results.
	if !fits() {
		convo = capToolResults(convo, trimmedToolChars)
	}
	return append(sys, convo...)
}

func firstUserIndex(msgs []codeMessage) int {
	for i, m := range msgs {
		if m.Role == "user" {
			return i
		}
	}
	return -1
}

// nextAssistant returns the index of the first assistant message at or after
// from, or -1.
func nextAssistant(msgs []codeMessage, from int) int {
	if from < 0 {
		return -1
	}
	for i := from; i < len(msgs); i++ {
		if msgs[i].Role == "assistant" {
			return i
		}
	}
	return -1
}

// capToolResults returns msgs with each tool result cut to limit characters.
// The input is not modified.
func capToolResults(msgs []codeMessage, limit int) []codeMessage {
	out := make([]codeMessage, len(msgs))
	for i, m := range msgs {
		if m.Role == "tool" {
			m.Content = tools.CapOutput(m.Content, limit)
		}
		out[i] = m
	}
	return out
}
