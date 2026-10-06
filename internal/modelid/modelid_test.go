package modelid

import (
	"slices"
	"testing"
)

func TestPublicIDsInclude55AndLegacy(t *testing.T) {
	got := PublicIDs()
	for _, want := range []string{Sonnet46, Haiku45, Sonnet5, Sonnet55, Opus55} {
		if !slices.Contains(got, want) {
			t.Fatalf("PublicIDs missing %s: %v", want, got)
		}
	}
	if got[0] != Default || Default != Sonnet46 {
		t.Fatalf("default/list order changed: first=%s default=%s", got[0], Default)
	}
}

func TestListedIDsIncludeThinkingVariants(t *testing.T) {
	got := ListedIDs()
	for _, want := range []string{
		Sonnet55, Sonnet55 + ThinkingSuffix,
		Opus55, Opus55 + ThinkingSuffix,
		Sonnet46, Sonnet46 + ThinkingSuffix,
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("ListedIDs missing %s: %v", want, got)
		}
	}
}

func TestResolveClaude55Aliases(t *testing.T) {
	cases := []struct {
		in       string
		id       string
		upstream string
		thinking bool
	}{
		{"", Sonnet46, Sonnet46, false},
		{"claude-sonnet-5-5", Sonnet55, ClaudeAISonnet55, false},
		{"claude-sonnet-5.5", Sonnet55, ClaudeAISonnet55, false},
		{"sonnet-5-5", Sonnet55, ClaudeAISonnet55, false},
		{"sonnet-5.5", Sonnet55, ClaudeAISonnet55, false},
		{"claude-sonnet-5-5-thinking", Sonnet55, ClaudeAISonnet55, true},
		{"claude-sonnet-5.5-thinking", Sonnet55, ClaudeAISonnet55, true},
		{"claude-sonnet-5-5-20260928", Sonnet55, ClaudeAISonnet55, false},
		{"claude-sonnet-5-5@20260928", Sonnet55, ClaudeAISonnet55, false},
		{"claude-sonnet-5-5[1m]", Sonnet55, ClaudeAISonnet55, false},
		{"anthropic.claude-sonnet-5-5", Sonnet55, ClaudeAISonnet55, false},
		{"us.anthropic.claude-sonnet-5-5-v1:0", Sonnet55, ClaudeAISonnet55, false},
		{"global.anthropic.claude-sonnet-5-5", Sonnet55, ClaudeAISonnet55, false},
		{"claude-opus-5-5", Opus55, ClaudeAIOpus55, false},
		{"claude-opus-5.5", Opus55, ClaudeAIOpus55, false},
		{"opus-5-5", Opus55, ClaudeAIOpus55, false},
		{"opus-5.5-thinking", Opus55, ClaudeAIOpus55, true},
		{"claude-opus-5-5-20260922-thinking", Opus55, ClaudeAIOpus55, true},
		{"anthropic.claude-opus-5-5", Opus55, ClaudeAIOpus55, false},
		{"eu.anthropic.claude-opus-5-5-v1:0", Opus55, ClaudeAIOpus55, false},
	}
	for _, tc := range cases {
		got := Resolve(tc.in)
		if !got.Known || got.ID != tc.id || got.Upstream != tc.upstream || got.Thinking != tc.thinking {
			t.Fatalf("Resolve(%q)=%+v want id=%s upstream=%s thinking=%v",
				tc.in, got, tc.id, tc.upstream, tc.thinking)
		}
		wantPublic := tc.id
		if tc.thinking {
			wantPublic += ThinkingSuffix
		}
		if got.Public != wantPublic {
			t.Fatalf("Resolve(%q).Public=%q want %q", tc.in, got.Public, wantPublic)
		}
	}
}

func TestResolveLegacyUnchanged(t *testing.T) {
	cases := []struct {
		in, id string
		think  bool
	}{
		{"claude-sonnet-4-6", Sonnet46, false},
		{"claude-sonnet-4-6-thinking", Sonnet46, true},
		{"claude-sonnet-4.6", Sonnet46, false},
		{"claude-haiku-4-5-20251001", Haiku45, false},
		{"claude-haiku-4-5", Haiku45, false},
		{"claude-sonnet-5", Sonnet5, false},
		{"claude-sonnet-5-thinking", Sonnet5, true},
	}
	for _, tc := range cases {
		got := Resolve(tc.in)
		if !got.Known || got.ID != tc.id || got.Upstream != tc.id || got.Thinking != tc.think {
			t.Fatalf("Resolve(%q)=%+v want id/upstream=%s thinking=%v", tc.in, got, tc.id, tc.think)
		}
	}
}

func TestResolveUnknownPassthrough(t *testing.T) {
	got := Resolve("claude-opus-4-6")
	if got.Known || got.Upstream != "claude-opus-4-6" || got.Thinking || got.Public != "claude-opus-4-6" {
		t.Fatalf("unknown model should pass through: %+v", got)
	}

	got = Resolve("custom-model-thinking")
	if got.Known || got.Upstream != "custom-model" || !got.Thinking || got.Public != "custom-model-thinking" {
		t.Fatalf("unknown thinking model should keep historic strip: %+v", got)
	}

	// Generic short aliases must not silently jump to 5.5.
	got = Resolve("sonnet")
	if got.Known || got.Upstream != "sonnet" {
		t.Fatalf("bare sonnet must not map to 5.5: %+v", got)
	}
	got = Resolve("opus")
	if got.Known || got.Upstream != "opus" {
		t.Fatalf("bare opus must not map to 5.5: %+v", got)
	}
}

func TestResolveDoesNotCollapseSonnet5To55(t *testing.T) {
	got := Resolve("claude-sonnet-5")
	if got.ID != Sonnet5 {
		t.Fatalf("claude-sonnet-5 must stay Sonnet 5, got %+v", got)
	}
	got = Resolve("claude-opus-5")
	if got.Known {
		t.Fatalf("claude-opus-5 is not in the catalog and must pass through: %+v", got)
	}
}
