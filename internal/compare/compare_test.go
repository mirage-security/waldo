package compare

import (
	"testing"

	"github.com/mirage-security/waldo/internal/model"
)

func TestReportsFailsOnlyNewlyFailingChanges(t *testing.T) {
	existingError := finding("existing", model.SeverityError, model.DispositionUnresolved)
	acceptedBecomesError := finding("changed", model.SeverityError, model.DispositionAccepted)
	base := report(existingError, acceptedBecomesError)
	head := report(
		existingError,
		finding("changed", model.SeverityError, model.DispositionUnresolved),
		finding("new-warning", model.SeverityWarning, model.DispositionUnresolved),
		finding("new-error", model.SeverityError, model.DispositionUnresolved),
	)
	result := Reports(base, head)
	if result.SchemaVersion != model.ReportSchemaVersion || result.BaseAnalysis.CodeFacts != 2 || result.HeadAnalysis.CodeFacts != 4 {
		t.Fatalf("comparison did not preserve analysis accounting: %#v", result)
	}
	if result.Failing != 2 {
		t.Fatalf("got %d failing changes, want 2", result.Failing)
	}
	if len(result.Introduced) != 2 || len(result.Changed) != 1 || result.Unchanged != 1 {
		t.Fatalf("unexpected comparison: %#v", result)
	}
}

func TestReportsRecordsResolvedFindings(t *testing.T) {
	base := report(finding("gone", model.SeverityWarning, model.DispositionAccepted))
	result := Reports(base, report())
	if len(result.Resolved) != 1 || result.Resolved[0].ID != "gone" || result.Failing != 0 {
		t.Fatalf("unexpected comparison: %#v", result)
	}
}

func finding(id string, severity model.Severity, disposition model.Disposition) model.Finding {
	return model.Finding{ID: id, Severity: severity, Disposition: disposition}
}

func report(findings ...model.Finding) model.Report {
	return model.Report{
		SchemaVersion: model.ReportSchemaVersion,
		Analysis:      model.Analysis{Input: model.AnalysisInputProviders, CodeFacts: len(findings)},
		Findings:      findings,
		Summary:       model.Summarize(findings),
	}
}
