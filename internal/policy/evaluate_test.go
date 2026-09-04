package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
	"github.com/mirage-security/waldo/internal/provider"
	"gopkg.in/yaml.v3"
)

type expectation struct {
	PolicyID    string            `yaml:"policyId"`
	Severity    model.Severity    `yaml:"severity"`
	Disposition model.Disposition `yaml:"disposition"`
	Reason      string            `yaml:"reason"`
	Findings    int               `yaml:"findings"`
	Failing     int               `yaml:"failing"`
}

func TestSemanticScenarios(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	scenariosRoot := filepath.Join(repositoryRoot, "testdata", "scenarios")
	entries, err := os.ReadDir(scenariosRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			configuration, err := config.Load(filepath.Join(repositoryRoot, "testdata", "waldo.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(scenariosRoot, entry.Name())
			facts, err := provider.LoadFacts(filepath.Join(directory, "facts.jsonl"), repositoryRoot)
			if err != nil {
				t.Fatal(err)
			}
			expectedBytes, err := os.ReadFile(filepath.Join(directory, "expect.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var expected expectation
			if err := yaml.Unmarshal(expectedBytes, &expected); err != nil {
				t.Fatal(err)
			}
			if expected.Disposition == model.DispositionAccepted || expected.Disposition == model.DispositionFalsePositive {
				if len(facts) != 1 {
					t.Fatal("disposition scenario must contain exactly one fact")
				}
				configuration.Dispositions = append(configuration.Dispositions, config.FindingDisposition{
					Finding:     FindingID(expected.PolicyID, "worker", facts[0].Provider, facts[0].ID),
					Disposition: expected.Disposition,
					Reason:      expected.Reason,
				})
			}

			findings, err := Evaluate(configuration, facts)
			if err != nil {
				t.Fatal(err)
			}
			summary := model.Summarize(findings)
			if len(findings) != expected.Findings {
				t.Fatalf("got %d findings, want %d", len(findings), expected.Findings)
			}
			if summary.Failing != expected.Failing {
				t.Fatalf("got %d failing, want %d", summary.Failing, expected.Failing)
			}
			if len(findings) == 1 {
				if findings[0].PolicyID != expected.PolicyID || findings[0].Severity != expected.Severity || findings[0].Disposition != expected.Disposition {
					t.Fatalf("unexpected finding: %#v", findings[0])
				}
				if findings[0].DispositionReason != expected.Reason {
					t.Fatalf("got reason %q, want %q", findings[0].DispositionReason, expected.Reason)
				}
			}
		})
	}
}

func TestFindingIDDoesNotDependOnLocation(t *testing.T) {
	first := FindingID("durable-work", "worker", "structural", "deferred:expiry")
	second := FindingID("durable-work", "worker", "structural", "deferred:expiry")
	if first != second {
		t.Fatalf("stable inputs yielded different IDs: %q and %q", first, second)
	}
	if first == FindingID("durable-work", "worker", "structural", "deferred:another-subject") {
		t.Fatal("different semantic fact IDs yielded the same finding ID")
	}
}

func TestDuplicateProviderIdentityFails(t *testing.T) {
	configuration, err := config.Load(filepath.Join("..", "..", "testdata", "waldo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fact := model.CodeFact{
		ID: "deferred:duplicate", Provider: "fixture", Kind: "deferred-execution",
		Source:     model.SourceLocation{Path: "src/work.example"},
		Attributes: map[string]any{"correctness.critical": true, "execution.authority": "process-local"},
	}
	if _, err := Evaluate(configuration, []model.CodeFact{fact, fact}); err == nil {
		t.Fatal("expected duplicate identity error")
	}
}
