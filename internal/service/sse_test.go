package service

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCompletionSSERefusal(t *testing.T) {
	raw := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"refusal\",\"stop_details\":{\"type\":\"refusal\",\"category\":\"reasoning_extraction\",\"explanation\":\"flagged\"}}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	err := parseCompletionSSE(strings.NewReader(raw), func(string) {})
	var refusal *RefusalError
	if !errors.As(err, &refusal) || refusal.Category != "reasoning_extraction" {
		t.Fatalf("want RefusalError, got %v", err)
	}
}
