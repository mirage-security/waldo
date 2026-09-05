package model

import (
	"time"

	"github.com/mirage-security/waldo/protocol"
)

const (
	// ConfigurationSchemaVersion is the waldo.yaml schema. It is intentionally
	// independent from policy documents and process protocols.
	ConfigurationSchemaVersion = 2
	// PolicySchemaVersion is the shared policy-document schema.
	PolicySchemaVersion = 1
	// ReportSchemaVersion is independent from the provider protocol and
	// configuration schemas. Version 3 names deployments directly and records
	// deployment-adapter accounting.
	ReportSchemaVersion = 3

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
	ID                string      `json:"id"`
	PolicyID          string      `json:"policyId"`
	PolicyTitle       string      `json:"policyTitle"`
	Severity          Severity    `json:"severity"`
	Disposition       Disposition `json:"disposition"`
	DispositionReason string      `json:"dispositionReason,omitempty"`
	Deployment        string      `json:"deployment,omitempty"`
	// DeploymentUnit is retained only so schema-v1/v2 reports can be read for
	// comparison. New reports leave it empty.
	DeploymentUnit    string         `json:"deploymentUnit,omitempty"`
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

type DeploymentAdapterRun struct {
	Deployment string `json:"deployment"`
	Adapter    string `json:"adapter"`
	Facts      int    `json:"facts"`
}

type Analysis struct {
	Input                 string                 `json:"input"`
	ProviderRuns          []ProviderRun          `json:"providerRuns"`
	DeploymentAdapterRuns []DeploymentAdapterRun `json:"deploymentAdapterRuns,omitempty"`
	CodeFacts             int                    `json:"codeFacts"`
	Deployments           int                    `json:"deployments,omitempty"`
	// DeploymentUnits is retained only for schema-v1/v2 report input.
	DeploymentUnits int `json:"deploymentUnits,omitempty"`
	Policies        int `json:"policies"`
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
