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
      source:
        root: src
        entrypoint: worker.ts
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

func TestDecodePreservesDeploymentSource(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    api-http:
      source:
        root: services/api
        entrypoint: src/http.ts
      facts: {}
`
	configuration, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	source := configuration.Deployment.Units["api-http"].Source
	if source.Root != "services/api" || source.Entrypoint != "src/http.ts" {
		t.Fatalf("unexpected deployment source: %#v", source)
	}
}

func TestDecodeRejectsInvalidDeploymentSource(t *testing.T) {
	tests := []struct {
		name       string
		sourceYAML string
		want       string
	}{
		{name: "missing root", sourceYAML: "{}", want: "source.root"},
		{name: "absolute root", sourceYAML: "{root: /services/api}", want: "source.root"},
		{name: "escaping root", sourceYAML: "{root: ../api}", want: "source.root"},
		{name: "absolute entrypoint", sourceYAML: "{root: services/api, entrypoint: /src/http.ts}", want: "source.entrypoint"},
		{name: "escaping entrypoint", sourceYAML: "{root: services/api, entrypoint: ../worker.ts}", want: "source.entrypoint"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := "version: 1\ndeployment:\n  units:\n    worker:\n      source: " + test.sourceYAML + "\n      facts: {}\n"
			_, err := Decode(strings.NewReader(input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q validation error, got %v", test.want, err)
			}
		})
	}
}

func TestDecodeExplicitPoliciesOverrideBuiltIns(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      source:
        root: src
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
      source:
        root: src
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
      source:
        root: src
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
