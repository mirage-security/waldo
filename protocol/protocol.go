// Package protocol defines the analyzer-neutral process contracts shared by
// Waldo, code-fact providers, and deployment adapters.
package protocol

const Version = 1

const DeploymentAdapterVersion = 1

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

// DeploymentRequest is written as one JSON object to a deployment adapter.
// Source is an absolute path resolved from the model's from.source value.
type DeploymentRequest struct {
	ProtocolVersion int            `json:"protocolVersion"`
	Root            string         `json:"root"`
	Source          string         `json:"source"`
	Resource        string         `json:"resource"`
	Options         map[string]any `json:"options,omitempty"`
}

// DeploymentResult is the single JSON object emitted by a deployment adapter.
// Facts are objective, analyzer-neutral properties of the selected resource.
type DeploymentResult struct {
	Facts map[string]any `json:"facts"`
}
