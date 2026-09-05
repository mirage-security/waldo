package model

import (
	"time"

	"github.com/mirage-security/waldo/protocol"
)

const (
	// SchemaVersion is the deployment-model and policy-document schema.
	SchemaVersion = protocol.Version
	// ReportSchemaVersion is independent from the provider protocol and
	// configuration schemas. Version 2 adds analysis accounting.
	ReportSchemaVersion = 2

	AnalysisInputProviders = "providers"
	AnalysisInputFactsFile = "facts-file"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

func (s Severity) Valid() bool {
	return s == SeverityError || s == SeverityWarning || s == SeverityInfo
}

type Disposition string

const (
	DispositionUnresolved    Disposition = "unresolved"
	DispositionAccepted      Disposition = "accepted"
	DispositionFalsePositive Disposition = "false-positive"
)

func (d Disposition) Valid() bool {
	return d == DispositionUnresolved || d == DispositionAccepted || d == DispositionFalsePositive
}

type SourceLocation = protocol.SourceLocation
type CodeFact = protocol.CodeFact

type Finding struct {
	ID                string         `json:"id"`
	PolicyID          string         `json:"policyId"`
	PolicyTitle       string         `json:"policyTitle"`
	Severity          Severity       `json:"severity"`
	Disposition       Disposition    `json:"disposition"`
	DispositionReason string         `json:"dispositionReason,omitempty"`
	DeploymentUnit    string         `json:"deploymentUnit"`
	MatchedDeployment map[string]any `json:"matchedDeploymentFacts"`
	CodeFact          CodeFact       `json:"codeFact"`
	Message           string         `json:"message"`
}

func (f Finding) FailsCI() bool {
	return f.Severity == SeverityError && f.Disposition == DispositionUnresolved
}

type Summary struct {
	Total          int `json:"total"`
	Errors         int `json:"errors"`
	Warnings       int `json:"warnings"`
	Info           int `json:"info"`
	Unresolved     int `json:"unresolved"`
	Accepted       int `json:"accepted"`
	FalsePositives int `json:"falsePositives"`
	Failing        int `json:"failing"`
}

type ProviderRun struct {
	Name      string `json:"name"`
	CodeFacts int    `json:"codeFacts"`
}

type Analysis struct {
	Input           string        `json:"input"`
	ProviderRuns    []ProviderRun `json:"providerRuns"`
	CodeFacts       int           `json:"codeFacts"`
	DeploymentUnits int           `json:"deploymentUnits"`
	Policies        int           `json:"policies"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Root          string    `json:"root"`
	Analysis      Analysis  `json:"analysis"`
	Findings      []Finding `json:"findings"`
	Summary       Summary   `json:"summary"`
}

func Summarize(findings []Finding) Summary {
	s := Summary{Total: len(findings)}
	for _, finding := range findings {
		switch finding.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		case SeverityInfo:
			s.Info++
		}
		switch finding.Disposition {
		case DispositionUnresolved:
			s.Unresolved++
		case DispositionAccepted:
			s.Accepted++
		case DispositionFalsePositive:
			s.FalsePositives++
		}
		if finding.FailsCI() {
			s.Failing++
		}
	}
	return s
}
