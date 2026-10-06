package service

import (
	"testing"

	"claude2api/internal/modelid"
)

func TestResolveUpstreamClaude55(t *testing.T) {
	model, think := resolveUpstream("claude-sonnet-5.5-thinking")
	if model != modelid.ClaudeAISonnet55 || !think {
		t.Fatalf("got model=%q think=%v", model, think)
	}
	model, think = resolveUpstream("claude-opus-5-5")
	if model != modelid.ClaudeAIOpus55 || think {
		t.Fatalf("got model=%q think=%v", model, think)
	}
	model, think = resolveUpstream("claude-sonnet-4-6-thinking")
	if model != modelid.Sonnet46 || !think {
		t.Fatalf("legacy thinking broken: model=%q think=%v", model, think)
	}
}

func TestApplyUpstreamOverride(t *testing.T) {
	overrides := map[string]string{modelid.Opus55: "claude-opus-5-5-web"}
	if got := applyUpstreamOverride(modelid.Opus55, modelid.ClaudeAIOpus55, overrides); got != "claude-opus-5-5-web" {
		t.Fatalf("override not applied: %q", got)
	}
	if got := applyUpstreamOverride(modelid.Sonnet55, modelid.ClaudeAISonnet55, overrides); got != modelid.ClaudeAISonnet55 {
		t.Fatalf("unrelated model should keep default slug: %q", got)
	}
	if got := applyUpstreamOverride(modelid.Opus55, modelid.ClaudeAIOpus55, nil); got != modelid.ClaudeAIOpus55 {
		t.Fatalf("nil map must not change slug: %q", got)
	}
}
