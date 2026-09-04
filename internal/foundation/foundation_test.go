package foundation_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
	"github.com/mirage-security/waldo/internal/policy"
	"github.com/mirage-security/waldo/internal/provider"
)

func TestOneInvariantAcrossTwoDeploymentModels(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	models := []string{
		"examples/durable-deferred-work/container.waldo.yaml",
		"examples/durable-deferred-work/function.waldo.yaml",
	}

	var firstPolicy config.Policy
	var firstPolicyFiles []string
	var firstFindingID string
	var executionModels []any
	for index, modelPath := range models {
		configuration, err := config.Load(filepath.Join(root, modelPath))
		if err != nil {
			t.Fatalf("load %s: %v", modelPath, err)
		}
		if len(configuration.Policies) != 1 {
			t.Fatalf("%s loaded %d policies, want 1", modelPath, len(configuration.Policies))
		}
		if index == 0 {
			firstPolicy = configuration.Policies[0]
			firstPolicyFiles = append([]string(nil), configuration.PolicyFiles...)
		} else if !reflect.DeepEqual(firstPolicy, configuration.Policies[0]) {
			t.Fatalf("deployment model changed the shared policy:\nfirst: %#v\nnext: %#v", firstPolicy, configuration.Policies[0])
		} else if !reflect.DeepEqual(firstPolicyFiles, configuration.PolicyFiles) {
			t.Fatalf("deployment models reference different policy files: %#v and %#v", firstPolicyFiles, configuration.PolicyFiles)
		}

		unit := configuration.Deployment.Units["expiry-notifier"]
		executionModels = append(executionModels, unit.Facts["platform.executionModel"])
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		facts, err := provider.Collect(ctx, root, configuration.Providers)
		cancel()
		if err != nil {
			t.Fatalf("collect facts for %s: %v", modelPath, err)
		}
		exampleFacts := 0
		criticalFacts := 0
		for _, fact := range facts {
			if !strings.HasPrefix(fact.Source.Path, "examples/durable-deferred-work/app/") {
				continue
			}
			exampleFacts++
			if fact.Attributes["correctness.critical"] == true {
				criticalFacts++
			}
		}
		if exampleFacts != 2 || criticalFacts != 1 {
			t.Fatalf("%s provider emitted %d example facts with %d critical; want 2 and 1", modelPath, exampleFacts, criticalFacts)
		}
		findings, err := policy.Evaluate(configuration, facts)
		if err != nil {
			t.Fatalf("evaluate %s: %v", modelPath, err)
		}
		if len(findings) != 1 {
			t.Fatalf("%s produced %d findings, want 1: %#v", modelPath, len(findings), findings)
		}
		finding := findings[0]
		if finding.PolicyID != "durable-deferred-execution" || finding.Severity != model.SeverityError || !finding.FailsCI() {
			t.Fatalf("%s produced the wrong finding: %#v", modelPath, finding)
		}
		if finding.CodeFact.Attributes["language"] != "go" || finding.CodeFact.Attributes["execution.authority"] != "process-local" || finding.CodeFact.Attributes["execution.mechanism"] != "process-local-timer" {
			t.Fatalf("%s did not use the source analyzer fact: %#v", modelPath, finding.CodeFact)
		}
		if index == 0 {
			firstFindingID = finding.ID
		} else if finding.ID != firstFindingID {
			t.Fatalf("same invariant and subject produced different IDs: %q and %q", firstFindingID, finding.ID)
		}
	}
	if reflect.DeepEqual(executionModels[0], executionModels[1]) {
		t.Fatalf("expected distinct deployment models, got %#v", executionModels)
	}
}
