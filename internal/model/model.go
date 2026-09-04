package model

import "time"

const SchemaVersion = 1

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

type SourceLocation struct {
	Path   string `json:"path" yaml:"path"`
	Line   int    `json:"line,omitempty" yaml:"line,omitempty"`
	Column int    `json:"column,omitempty" yaml:"column,omitempty"`
}

// CodeFact is the provider-neutral record consumed by Waldo. ID is a stable,
// provider-owned semantic identity; it must not depend on a source line number.
type CodeFact struct {
	ID         string         `json:"id" yaml:"id"`
	Provider   string         `json:"provider,omitempty" yaml:"provider,omitempty"`
	Kind       string         `json:"kind" yaml:"kind"`
	Source     SourceLocation `json:"source" yaml:"source"`
	Symbol     string         `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}

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

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Root          string    `json:"root"`
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
