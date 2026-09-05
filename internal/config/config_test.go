package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesSharedPolicyFiles(t *testing.T) {
	configuration, err := Load(filepath.Join("..", "..", "examples", "durable-deferred-work", "container.waldo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.PolicyFiles) != 1 || len(configuration.Policies) != 1 {
		t.Fatalf("unexpected shared policies: %#v", configuration)
	}
}

func TestDecodeLoadsBuiltInPoliciesForTopologyOnlyModel(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts:
        process.restartable: true
`
	configuration, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"durable-deferred-execution",
		"non-durable-deferred-execution",
		"process-local-coordination",
		"replica-local-authority",
	}
	if len(configuration.Policies) != len(want) {
		t.Fatalf("loaded %d built-in policies, want %d: %#v", len(configuration.Policies), len(want), configuration.Policies)
	}
	for index, policy := range configuration.Policies {
		if policy.ID != want[index] {
			t.Fatalf("policy %d is %q, want %q", index, policy.ID, want[index])
		}
	}
}

func TestDecodeExplicitPoliciesOverrideBuiltIns(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts: {}
policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
`
	configuration, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.Policies) != 1 || configuration.Policies[0].ID != "example" {
		t.Fatalf("explicit policy set did not override built-ins: %#v", configuration.Policies)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts: {}
policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
surprise: true
`
	_, err := Decode(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestDispositionRequiresReason(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts: {}
policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
dispositions:
  - finding: waldo:v1:0000000000000000000000000000000000000000000000000000000000000000
    disposition: accepted
    reason: ""
`
	_, err := Decode(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "must include a reason") {
		t.Fatalf("expected reason validation error, got %v", err)
	}
}
