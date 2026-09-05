package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/deployment"
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
			if _, err := deployment.Resolve(context.Background(), repositoryRoot, &configuration); err != nil {
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
					Finding:     FindingID(expected.PolicyID, "fixture/worker", facts[0].Provider, facts[0].ID),
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

func TestMatchingDeploymentsConservativelyIncludesSameArtifactEntrypoints(t *testing.T) {
	configuration := config.Config{
		Service: "api",
		Artifacts: map[string]config.Artifact{
			"server":    {ResolvedSource: "services/api", Entrypoint: "src/http.ts"},
			"worker":    {ResolvedSource: "services/api", Entrypoint: "src/worker.ts"},
			"reporting": {ResolvedSource: "services/reporting", Entrypoint: "src/http.ts"},
		},
		Deployments: map[string]config.Deployment{
			"api-http":       {Artifact: "server"},
			"api-worker":     {Artifact: "worker"},
			"reporting-http": {Artifact: "reporting"},
		},
	}

	matches := matchingDeployments(configuration, "services/api/src/shared/state.ts")
	if len(matches) != 2 || matches[0] != "api-http" || matches[1] != "api-worker" {
		t.Fatalf("unexpected conservative source matches: %#v", matches)
	}
}

func TestDuplicateProviderIdentityFails(t *testing.T) {
	configuration, err := config.Load(filepath.Join("..", "..", "testdata", "waldo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deployment.Resolve(context.Background(), filepath.Join("..", ".."), &configuration); err != nil {
		t.Fatal(err)
	}
	fact := model.CodeFact{
		ID: "deferred:duplicate", Provider: "fixture", Kind: "deferred-execution",
		Source:     model.SourceLocation{Path: "testdata/src/work.example"},
		Attributes: map[string]any{"correctness.critical": true, "execution.authority": "process-local"},
	}
	if _, err := Evaluate(configuration, []model.CodeFact{fact, fact}); err == nil {
		t.Fatal("expected duplicate identity error")
	}
}

func TestProcessLocalCoordinationRequiresBothBoundaries(t *testing.T) {
	fact := model.CodeFact{
		ID:       "coordination:handoff",
		Provider: "fixture",
		Kind:     "coordination",
		Source:   model.SourceLocation{Path: "src/state.example"},
		Attributes: map[string]any{
			"coordination.authority":     "process-local",
			"coordination.confidence":    "high",
			"coordination.requiredScope": "deployment",
			"coordination.scope":         "cross-request",
		},
	}
	rule := config.Policy{
		ID:       "process-local-coordination",
		Title:    "Process-local authority cannot coordinate concurrent instances",
		Severity: model.SeverityError,
		When: config.Conditions{
			Deployment: map[string]any{
				"deployment.replicas.concurrent": true,
				"memory.scope":                   "instance",
			},
			Code: config.CodeConditions{Kind: "coordination", Attributes: map[string]any{
				"coordination.authority":     "process-local",
				"coordination.confidence":    "high",
				"coordination.requiredScope": "deployment",
				"coordination.scope":         "cross-request",
			}},
		},
		Message: "Cross-request coordination relies on instance-local authority.",
	}

	tests := []struct {
		name        string
		facts       []model.CodeFact
		concurrent  bool
		memoryScope string
		want        int
	}{
		{name: "both match", facts: []model.CodeFact{fact}, concurrent: true, memoryScope: "instance", want: 1},
		{name: "code missing", concurrent: true, memoryScope: "instance", want: 0},
		{name: "single replica", facts: []model.CodeFact{fact}, concurrent: false, memoryScope: "instance", want: 0},
		{name: "shared memory", facts: []model.CodeFact{fact}, concurrent: true, memoryScope: "deployment", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration := config.Config{
				Service:  "fixture",
				Policies: []config.Policy{rule},
				Artifacts: map[string]config.Artifact{
					"worker": {ResolvedSource: "src", Entrypoint: "worker.ts"},
				},
				Deployments: map[string]config.Deployment{
					"worker": {
						Artifact: "worker",
						Facts: map[string]any{
							"deployment.replicas.concurrent": test.concurrent,
							"memory.scope":                   test.memoryScope,
						},
					},
				},
			}
			findings, err := Evaluate(configuration, test.facts)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d", len(findings), test.want)
			}
		})
	}
}

func TestProcessLocalAuthorityRequiresBothBoundaries(t *testing.T) {
	fact := model.CodeFact{
		ID:       "authority:local-state",
		Provider: "fixture",
		Kind:     "state-dependent-decision",
		Source:   model.SourceLocation{Path: "src/state.example"},
		Attributes: map[string]any{
			"state.role":  "unknown",
			"state.scope": "process-local",
		},
	}
	rule := config.Policy{
		ID:       "process-local-authority",
		Title:    "Application behavior may depend on state held by one process",
		Severity: model.SeverityWarning,
		When: config.Conditions{
			Deployment: map[string]any{
				"memory.scope":        "instance",
				"process.restartable": true,
			},
			Code: config.CodeConditions{Kind: "state-dependent-decision", Attributes: map[string]any{
				"state.role":  map[string]any{"oneOf": []any{"authority", "unknown"}},
				"state.scope": "process-local",
			}},
		},
		Message: "Application behavior may depend on state held by one process.",
	}

	tests := []struct {
		name        string
		withFact    bool
		stateRole   string
		restartable bool
		memoryScope string
		want        int
	}{
		{name: "code and deployment match", withFact: true, stateRole: "unknown", restartable: true, memoryScope: "instance", want: 1},
		{name: "code missing", restartable: true, memoryScope: "instance"},
		{name: "process is not replaceable", withFact: true, stateRole: "unknown", memoryScope: "instance"},
		{name: "memory is shared", withFact: true, stateRole: "unknown", restartable: true, memoryScope: "deployment"},
		{name: "durably backed cache", withFact: true, stateRole: "cache", restartable: true, memoryScope: "instance"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration := config.Config{
				Service:  "fixture",
				Policies: []config.Policy{rule},
				Artifacts: map[string]config.Artifact{
					"server": {ResolvedSource: "src", Entrypoint: "server.ts"},
				},
				Deployments: map[string]config.Deployment{
					"production": {
						Artifact: "server",
						Facts: map[string]any{
							"memory.scope":        test.memoryScope,
							"process.restartable": test.restartable,
						},
					},
				},
			}
			var facts []model.CodeFact
			if test.withFact {
				configuredFact := fact
				configuredFact.Attributes = map[string]any{
					"state.role":  test.stateRole,
					"state.scope": "process-local",
				}
				facts = []model.CodeFact{configuredFact}
			}
			findings, err := Evaluate(configuration, facts)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d: %#v", len(findings), test.want, findings)
			}
		})
	}
}

func TestNonDurableDeferredExecutionRequiresBothBoundaries(t *testing.T) {
	configuration, err := config.Load(filepath.Join("..", "..", "testdata", "waldo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fact := model.CodeFact{
		ID:       "deferred:expiry",
		Provider: "fixture",
		Kind:     "deferred-execution",
		Source:   model.SourceLocation{Path: "testdata/src/expiry.example"},
		Attributes: map[string]any{
			"correctness.criticality": "unknown",
			"execution.authority":     "process-local",
		},
	}

	tests := []struct {
		name  string
		fact  *model.CodeFact
		facts map[string]any
		want  int
	}{
		{
			name: "code and deployment match",
			fact: &fact,
			facts: map[string]any{
				"process.restartable":             true,
				"scheduling.processLocal.durable": false,
			},
			want: 1,
		},
		{
			name: "code does not match",
			facts: map[string]any{
				"process.restartable":             true,
				"scheduling.processLocal.durable": false,
			},
		},
		{
			name: "deployment does not match",
			fact: &fact,
			facts: map[string]any{
				"process.restartable":             false,
				"scheduling.processLocal.durable": false,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration.Artifacts["worker"] = config.Artifact{ResolvedSource: "testdata", Entrypoint: "src/expiry.example"}
			configuredDeployment := configuration.Deployments["worker"]
			configuredDeployment.Facts = test.facts
			configuration.Deployments["worker"] = configuredDeployment
			var facts []model.CodeFact
			if test.fact != nil {
				facts = []model.CodeFact{*test.fact}
			}
			findings, err := Evaluate(configuration, facts)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != test.want {
				t.Fatalf("got %d findings, want %d: %#v", len(findings), test.want, findings)
			}
			if len(findings) == 1 {
				finding := findings[0]
				if finding.PolicyID != "non-durable-deferred-execution" || finding.Severity != model.SeverityWarning || finding.FailsCI() {
					t.Fatalf("unexpected finding: %#v", finding)
				}
				if finding.MatchedCode["correctness.criticality"] != "unknown" || finding.MatchedCode["execution.authority"] != "process-local" {
					t.Fatalf("matched code evidence is missing: %#v", finding.MatchedCode)
				}
				if finding.MatchedDeployment["process.restartable"] != true || finding.MatchedDeployment["scheduling.processLocal.durable"] != false {
					t.Fatalf("matched deployment evidence is missing: %#v", finding.MatchedDeployment)
				}
			}
		})
	}
}

func TestRecommendationDoesNotChangeFindingIdentity(t *testing.T) {
	configuration, err := config.Load(filepath.Join("..", "..", "testdata", "waldo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	configuration.Artifacts["worker"] = config.Artifact{ResolvedSource: "testdata", Entrypoint: "src/expiry.example"}
	configuredDeployment := configuration.Deployments["worker"]
	configuredDeployment.Facts = map[string]any{
		"process.restartable":             true,
		"scheduling.processLocal.durable": false,
	}
	configuration.Deployments["worker"] = configuredDeployment
	facts := []model.CodeFact{{
		ID:       "deferred:expiry",
		Provider: "fixture",
		Kind:     "deferred-execution",
		Source:   model.SourceLocation{Path: "testdata/src/expiry.example"},
		Attributes: map[string]any{
			"correctness.critical": true,
			"execution.authority":  "process-local",
		},
	}}

	withRecommendation, err := Evaluate(configuration, facts)
	if err != nil {
		t.Fatal(err)
	}
	delete(configuration.Recommendations, "durable-deferred-execution")
	withoutRecommendation, err := Evaluate(configuration, facts)
	if err != nil {
		t.Fatal(err)
	}
	if len(withRecommendation) != 1 || withRecommendation[0].Recommendation == nil {
		t.Fatalf("expected one recommended finding: %#v", withRecommendation)
	}
	if len(withoutRecommendation) != 1 || withoutRecommendation[0].Recommendation != nil {
		t.Fatalf("expected one finding without a recommendation: %#v", withoutRecommendation)
	}
	if withRecommendation[0].ID != withoutRecommendation[0].ID ||
		withRecommendation[0].Severity != withoutRecommendation[0].Severity ||
		withRecommendation[0].Disposition != withoutRecommendation[0].Disposition {
		t.Fatalf("recommendation changed invariant state: %#v %#v", withRecommendation[0], withoutRecommendation[0])
	}
}
