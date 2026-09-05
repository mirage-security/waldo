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
	if !filepath.IsAbs(configuration.BaseDir) {
		t.Fatalf("configuration base directory is not absolute: %q", configuration.BaseDir)
	}
}

func TestDecodeLoadsBuiltInPoliciesForBindingOnlyModel(t *testing.T) {
	configuration, err := Decode(strings.NewReader(validModel("")))
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

func TestDecodePreservesArtifactAndDeploymentBinding(t *testing.T) {
	configuration, err := Decode(strings.NewReader(validModel("")))
	if err != nil {
		t.Fatal(err)
	}
	artifact := configuration.Artifacts["server"]
	if artifact.Source != "src" || artifact.Entrypoint != "http.ts" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	deployment := configuration.Deployments["production"]
	if deployment.Artifact != "server" || deployment.From.Adapter != "terraform" || deployment.From.Source != "infra" || deployment.From.Resource != "module.service" {
		t.Fatalf("unexpected deployment binding: %#v", deployment)
	}
	varFiles, ok := deployment.From.With["varFiles"].([]any)
	if !ok || len(varFiles) != 1 || varFiles[0] != "production.tfvars" {
		t.Fatalf("unexpected adapter options: %#v", deployment.From.With)
	}
}

func TestDecodeRejectsInvalidModel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "old schema", input: strings.Replace(validModel(""), "version: 2", "version: 1", 1), want: "version must be 2"},
		{name: "missing service", input: strings.Replace(validModel(""), "service: reporting", "service: ''", 1), want: "service cannot be empty"},
		{name: "absolute artifact source", input: strings.Replace(validModel(""), "source: src", "source: /src", 1), want: "must be relative to waldo.yaml"},
		{name: "absolute entrypoint", input: strings.Replace(validModel(""), "entrypoint: http.ts", "entrypoint: /http.ts", 1), want: "must be relative to its source"},
		{name: "unknown artifact", input: strings.Replace(validModel(""), "artifact: server", "artifact: missing", 1), want: "references unknown artifact"},
		{name: "missing adapter", input: strings.Replace(validModel(""), "adapter: terraform", "adapter: ''", 1), want: "from.adapter"},
		{name: "absolute deployment source", input: strings.Replace(validModel(""), "source: infra", "source: /infra", 1), want: "from.source"},
		{name: "missing resource", input: strings.Replace(validModel(""), "resource: module.service", "resource: ''", 1), want: "from.resource"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q validation error, got %v", test.want, err)
			}
		})
	}
}

func TestDecodeExplicitPoliciesOverrideBuiltIns(t *testing.T) {
	extra := `policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
`
	configuration, err := Decode(strings.NewReader(validModel(extra)))
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.Policies) != 1 || configuration.Policies[0].ID != "example" {
		t.Fatalf("explicit policy set did not override built-ins: %#v", configuration.Policies)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := Decode(strings.NewReader(validModel("surprise: true\n")))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestDispositionRequiresReason(t *testing.T) {
	extra := `dispositions:
  - finding: waldo:v2:0000000000000000000000000000000000000000000000000000000000000000
    disposition: accepted
    reason: ""
`
	_, err := Decode(strings.NewReader(validModel(extra)))
	if err == nil || !strings.Contains(err.Error(), "must include a reason") {
		t.Fatalf("expected reason validation error, got %v", err)
	}
}

func validModel(extra string) string {
	return `version: 2
service: reporting
artifacts:
  server:
    source: src
    entrypoint: http.ts
deployments:
  production:
    artifact: server
    from:
      adapter: terraform
      source: infra
      resource: module.service
      with:
        varFiles: [production.tfvars]
` + extra
}
