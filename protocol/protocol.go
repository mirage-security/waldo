// Package protocol defines the analyzer-neutral process contracts shared by
// Waldo, code-fact providers, and deployment adapters.
package protocol

const Version = 1

// ProviderProtocolVersion is the current code-provider protocol. Version 1
// remains supported for facts-only external providers.
const ProviderProtocolVersion = 2

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

type ProviderCoverageStatus string

const (
	ProviderCoverageComplete ProviderCoverageStatus = "complete"
	ProviderCoveragePartial  ProviderCoverageStatus = "partial"
)

func (status ProviderCoverageStatus) Valid() bool {
	return status == ProviderCoverageComplete || status == ProviderCoveragePartial
}

// ProviderSummary closes a protocol-v2 stream and makes incomplete analysis
// distinguishable from a complete zero-fact run.
type ProviderSummary struct {
	Coverage              ProviderCoverageStatus `json:"coverage"`
	FilesAttempted        int                    `json:"filesAttempted,omitempty"`
	FilesNotFullyAnalyzed int                    `json:"filesNotFullyAnalyzed,omitempty"`
}

const (
	ProviderRecordFact    = "fact"
	ProviderRecordSummary = "summary"
)

// ProviderRecord is one JSONL record in a protocol-v2 provider stream.
type ProviderRecord struct {
	Type    string           `json:"type"`
	Fact    *CodeFact        `json:"fact,omitempty"`
	Summary *ProviderSummary `json:"summary,omitempty"`
}

type ProviderResult struct {
	Facts   []CodeFact
	Summary ProviderSummary
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
