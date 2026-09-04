// Package protocol defines the analyzer-neutral process protocol shared by
// Waldo and code-fact providers.
package protocol

const Version = 1

// Request is written as one JSON object to a provider's standard input.
type Request struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Root            string `json:"root"`
}

type SourceLocation struct {
	Path   string `json:"path"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// CodeFact is one JSONL record emitted by a provider. ID is a stable,
// provider-owned semantic identity; it must not depend on a source line.
type CodeFact struct {
	ID         string         `json:"id"`
	Provider   string         `json:"provider,omitempty"`
	Kind       string         `json:"kind"`
	Source     SourceLocation `json:"source"`
	Symbol     string         `json:"symbol,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
