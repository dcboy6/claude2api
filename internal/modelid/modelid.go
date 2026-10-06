// Package modelid is the single source of truth for public model IDs,
// client aliases, and the slugs forwarded to Claude.ai.
package modelid

import (
	"regexp"
	"strings"
)

const (
	// Default is the empty-model fallback. Kept as Sonnet 4.6 so existing
	// clients that omit `model` keep the same behavior.
	Default = Sonnet46

	Sonnet46 = "claude-sonnet-4-6"
	Haiku45  = "claude-haiku-4-5-20251001"
	Sonnet5  = "claude-sonnet-5"
	Sonnet55 = "claude-sonnet-5-5"
	Opus55   = "claude-opus-5-5"

	// ClaudeAISonnet55 and ClaudeAIOpus55 are the slugs sent to
	// claude.ai /chat_conversations and /completion.
	//
	// Inferred, not live-verified: this project already forwards the public
	// API IDs unchanged for claude-sonnet-4-6, claude-sonnet-5, and
	// claude-haiku-4-5-20251001. Anthropic's 4.6+ IDs are dateless snapshots
	// (claude-opus-5-5 / claude-sonnet-5-5), so the web API most likely
	// uses the same strings. Override via config.claude_ai_model_ids if a
	// live account rejects them.
	ClaudeAISonnet55 = "claude-sonnet-5-5"
	ClaudeAIOpus55   = "claude-opus-5-5"
)

// ThinkingSuffix is the project-wide switch for Claude.ai extended thinking
// (paprika_mode=extended). Same pattern as existing models.
const ThinkingSuffix = "-thinking"

// Spec describes one model the gateway exposes.
type Spec struct {
	// ID is the canonical public id listed by GET /v1/models.
	ID string
	// Upstream is the default Claude.ai web slug (no -thinking suffix).
	Upstream string
	// Aliases are additional ids clients may send. Matched after normalize.
	Aliases []string
}

// catalog is the ordered list of exposed base models. New families are
// appended so existing clients that pick data[0] still see Sonnet 4.6.
var catalog = []Spec{
	{
		ID:       Sonnet46,
		Upstream: Sonnet46,
		Aliases:  []string{"claude-sonnet-4.6", "sonnet-4-6", "sonnet-4.6"},
	},
	{
		ID:       Haiku45,
		Upstream: Haiku45,
		Aliases:  []string{"claude-haiku-4-5", "claude-haiku-4.5", "haiku-4-5", "haiku-4.5"},
	},
	{
		ID:       Sonnet5,
		Upstream: Sonnet5,
		Aliases:  []string{"claude-sonnet-5.0", "sonnet-5"},
	},
	{
		ID:       Sonnet55,
		Upstream: ClaudeAISonnet55,
		Aliases: []string{
			"claude-sonnet-5.5",
			"sonnet-5-5",
			"sonnet-5.5",
			"claude-sonnet-5-5-20260928",
		},
	},
	{
		ID:       Opus55,
		Upstream: ClaudeAIOpus55,
		Aliases: []string{
			"claude-opus-5.5",
			"opus-5-5",
			"opus-5.5",
			"claude-opus-5-5-20260922",
		},
	},
}

var (
	aliasIndex  = map[string]Spec{}
	datedSuffix = regexp.MustCompile(`-\d{8}$`)
)

func init() {
	for _, spec := range catalog {
		register(spec.ID, spec)
		for _, alias := range spec.Aliases {
			register(alias, spec)
		}
	}
}

func register(name string, spec Spec) {
	for _, key := range lookupKeys(name) {
		if key == "" {
			continue
		}
		if _, exists := aliasIndex[key]; !exists {
			aliasIndex[key] = spec
		}
	}
}

// PublicIDs returns canonical base model ids in catalog order.
func PublicIDs() []string {
	out := make([]string, len(catalog))
	for i, spec := range catalog {
		out[i] = spec.ID
	}
	return out
}

// ListedIDs returns the ids GET /v1/models exposes: each base model plus
// its -thinking variant, matching the existing listing pattern.
func ListedIDs() []string {
	base := PublicIDs()
	out := make([]string, 0, len(base)*2)
	for _, id := range base {
		out = append(out, id, id+ThinkingSuffix)
	}
	return out
}

// Resolved is a client model string after alias / thinking normalization.
type Resolved struct {
	// ID is the canonical public id. Empty when the request is unknown.
	ID string
	// Public is the id echoed to clients (canonical + optional -thinking).
	// Unknown requests keep the trimmed original string.
	Public string
	// Upstream is the Claude.ai slug, without -thinking.
	Upstream string
	// Thinking is true when the client asked for extended thinking.
	Thinking bool
	// Known is true when the request mapped to a catalog model.
	Known bool
}

// Resolve maps a client model id to the public id and Claude.ai slug.
// Unknown names are forwarded as-is (after stripping -thinking), which is
// the historic behavior and keeps older / custom ids working.
func Resolve(req string) Resolved {
	raw := strings.TrimSpace(req)
	if raw == "" {
		return known(Default, false)
	}

	key := strings.ToLower(raw)
	thinking := strings.HasSuffix(key, ThinkingSuffix)
	if thinking {
		key = strings.TrimSuffix(key, ThinkingSuffix)
	}
	key = strings.TrimSpace(key)

	if spec, ok := lookup(key); ok {
		return known(spec.ID, thinking)
	}

	upstream := raw
	if thinking && strings.HasSuffix(strings.ToLower(raw), ThinkingSuffix) {
		upstream = raw[:len(raw)-len(ThinkingSuffix)]
	}
	public := raw
	if thinking && !strings.HasSuffix(strings.ToLower(public), ThinkingSuffix) {
		public += ThinkingSuffix
	}
	return Resolved{Public: public, Upstream: strings.TrimSpace(upstream), Thinking: thinking}
}

func known(id string, thinking bool) Resolved {
	spec := mustSpec(id)
	public := spec.ID
	if thinking {
		public += ThinkingSuffix
	}
	return Resolved{
		ID:       spec.ID,
		Public:   public,
		Upstream: spec.Upstream,
		Thinking: thinking,
		Known:    true,
	}
}

func mustSpec(id string) Spec {
	if spec, ok := aliasIndex[normalizeLookup(id)]; ok {
		return spec
	}
	return Spec{ID: id, Upstream: id}
}

func lookup(raw string) (Spec, bool) {
	for _, key := range lookupKeys(raw) {
		if spec, ok := aliasIndex[key]; ok {
			return spec, true
		}
		if stripped, ok := stripDated(key); ok {
			if spec, ok := aliasIndex[stripped]; ok {
				return spec, true
			}
		}
	}
	return Spec{}, false
}

func lookupKeys(raw string) []string {
	norm := normalizeLookup(raw)
	if norm == "" {
		return nil
	}
	keys := []string{norm}
	if dotted := normalizeLookup(strings.ReplaceAll(raw, ".", "-")); dotted != "" && dotted != norm {
		keys = append(keys, dotted)
	}
	return keys
}

func normalizeLookup(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ThinkingSuffix)
	s = stripContextWindow(s)
	for _, prefix := range []string{"us.", "eu.", "global.", "anthropic."} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = stripBedrockVersion(s)
	s = strings.ReplaceAll(s, "@", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

func stripContextWindow(s string) string {
	s = strings.TrimSuffix(s, "[1m]")
	s = strings.TrimSuffix(s, "[1M]")
	s = strings.TrimSuffix(s, ":1m")
	return s
}

func stripBedrockVersion(s string) string {
	i := strings.LastIndex(s, "-v")
	if i <= 0 {
		return s
	}
	rest := s[i+2:]
	if rest == "" {
		return s
	}
	for _, r := range rest {
		if r != ':' && (r < '0' || r > '9') {
			return s
		}
	}
	return s[:i]
}

func stripDated(s string) (string, bool) {
	loc := datedSuffix.FindStringIndex(s)
	if loc == nil {
		return "", false
	}
	return s[:loc[0]], true
}

// SpecByID returns the catalog entry for a canonical public id.
func SpecByID(id string) (Spec, bool) {
	spec, ok := aliasIndex[normalizeLookup(id)]
	if !ok || spec.ID != id {
		return Spec{}, false
	}
	return spec, true
}
