package compare

import (
	"reflect"
	"sort"

	"github.com/mirage-security/waldo/internal/model"
)

type FindingChange struct {
	Base model.Finding `json:"base"`
	Head model.Finding `json:"head"`
}

type Result struct {
	SchemaVersion int             `json:"schemaVersion"`
	BaseSummary   model.Summary   `json:"baseSummary"`
	HeadSummary   model.Summary   `json:"headSummary"`
	BaseAnalysis  model.Analysis  `json:"baseAnalysis"`
	HeadAnalysis  model.Analysis  `json:"headAnalysis"`
	Introduced    []model.Finding `json:"introduced"`
	Resolved      []model.Finding `json:"resolved"`
	Changed       []FindingChange `json:"changed"`
	Unchanged     int             `json:"unchanged"`
	Failing       int             `json:"failing"`
}

func Reports(base, head model.Report) Result {
	result := Result{
		SchemaVersion: model.ReportSchemaVersion,
		BaseSummary:   base.Summary,
		HeadSummary:   head.Summary,
		BaseAnalysis:  base.Analysis,
		HeadAnalysis:  head.Analysis,
		Introduced:    make([]model.Finding, 0),
		Resolved:      make([]model.Finding, 0),
		Changed:       make([]FindingChange, 0),
	}
	baseByID := byID(base.Findings)
	headByID := byID(head.Findings)
	for id, headFinding := range headByID {
		baseFinding, existed := baseByID[id]
		if !existed {
			result.Introduced = append(result.Introduced, headFinding)
			if headFinding.FailsCI() {
				result.Failing++
			}
			continue
		}
		if findingStateEqual(baseFinding, headFinding) {
			result.Unchanged++
			continue
		}
		result.Changed = append(result.Changed, FindingChange{Base: baseFinding, Head: headFinding})
		if !baseFinding.FailsCI() && headFinding.FailsCI() {
			result.Failing++
		}
	}
	for id, baseFinding := range baseByID {
		if _, exists := headByID[id]; !exists {
			result.Resolved = append(result.Resolved, baseFinding)
		}
	}
	sort.Slice(result.Introduced, func(i, j int) bool { return result.Introduced[i].ID < result.Introduced[j].ID })
	sort.Slice(result.Resolved, func(i, j int) bool { return result.Resolved[i].ID < result.Resolved[j].ID })
	sort.Slice(result.Changed, func(i, j int) bool { return result.Changed[i].Head.ID < result.Changed[j].Head.ID })
	return result
}

func byID(findings []model.Finding) map[string]model.Finding {
	indexed := make(map[string]model.Finding, len(findings))
	for _, finding := range findings {
		indexed[finding.ID] = finding
	}
	return indexed
}

func findingStateEqual(left, right model.Finding) bool {
	return left.Severity == right.Severity &&
		left.Disposition == right.Disposition &&
		left.DispositionReason == right.DispositionReason &&
		reflect.DeepEqual(left.MatchedDeployment, right.MatchedDeployment) &&
		reflect.DeepEqual(left.CodeFact.Attributes, right.CodeFact.Attributes)
}
