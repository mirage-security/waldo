package deployment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/protocol"
)

func TestMain(m *testing.M) {
	if os.Getenv("WALDO_TEST_DEPLOYMENT_ADAPTER") == "1" {
		var request protocol.DeploymentRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil || request.Resource != "module.service" {
			os.Exit(3)
		}
		if err := json.NewEncoder(os.Stdout).Encode(protocol.DeploymentResult{Facts: map[string]any{"helper.completed": true}}); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestResolveFactsBinding(t *testing.T) {
	root := t.TempDir()
	serviceDir := filepath.Join(root, "services", "reporting")
	if err := os.MkdirAll(filepath.Join(serviceDir, "deployment"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(serviceDir, "deployment", "production.yaml"), `version: 1
resources:
  reporting:
    facts:
      process.restartable: true
      deployment.replicas.concurrent: true
`)
	configuration := config.Config{
		Service: "reporting",
		BaseDir: serviceDir,
		Artifacts: map[string]config.Artifact{
			"server": {Entrypoint: "src/index.ts"},
		},
		Deployments: map[string]config.Deployment{
			"production": {
				Artifact: "server",
				From: config.DeploymentReference{
					Adapter: "facts", Source: "deployment/production.yaml", Resource: "reporting",
				},
			},
		},
	}
	runs, err := Resolve(context.Background(), root, &configuration)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Artifacts["server"].ResolvedSource != "services/reporting" {
		t.Fatalf("unexpected resolved artifact source: %#v", configuration.Artifacts["server"])
	}
	facts := configuration.Deployments["production"].Facts
	if facts["process.restartable"] != true || facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("unexpected deployment facts: %#v", facts)
	}
	if len(runs) != 1 || runs[0].Deployment != "reporting/production" || runs[0].Adapter != "facts" || runs[0].Facts != 2 {
		t.Fatalf("unexpected adapter accounting: %#v", runs)
	}
}

func TestResolveRejectsArtifactOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "service"), 0o755); err != nil {
		t.Fatal(err)
	}
	configuration := config.Config{
		Service: "reporting",
		BaseDir: filepath.Join(root, "service"),
		Artifacts: map[string]config.Artifact{
			"server": {Source: "../..", Entrypoint: "index.ts"},
		},
	}
	_, err := Resolve(context.Background(), root, &configuration)
	if err == nil || !strings.Contains(err.Error(), "outside analysis root") {
		t.Fatalf("expected root-boundary error, got %v", err)
	}
}

func TestResolveExternalAdapterProtocol(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.tf")
	writeFile(t, source, "# adapter fixture\n")
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WALDO_TEST_DEPLOYMENT_ADAPTER", "1")
	configuration := config.Config{
		Service: "reporting", BaseDir: root,
		Artifacts: map[string]config.Artifact{"server": {Entrypoint: "index.ts"}},
		Deployments: map[string]config.Deployment{
			"production": {
				Artifact: "server",
				From:     config.DeploymentReference{Adapter: executable, Source: "main.tf", Resource: "module.service"},
			},
		},
	}
	runs, err := Resolve(context.Background(), root, &configuration)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Deployments["production"].Facts["helper.completed"] != true || len(runs) != 1 {
		t.Fatalf("external adapter did not resolve: %#v %#v", configuration.Deployments, runs)
	}
}

func TestResolveRejectsDeploymentSourceOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	serviceDir := filepath.Join(root, "service")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(parent, "outside.yaml"), "version: 1\nresources: {}\n")
	configuration := config.Config{
		Service: "reporting", BaseDir: serviceDir,
		Artifacts: map[string]config.Artifact{"server": {Entrypoint: "index.ts"}},
		Deployments: map[string]config.Deployment{
			"production": {
				Artifact: "server",
				From:     config.DeploymentReference{Adapter: "facts", Source: "../../outside.yaml", Resource: "service"},
			},
		},
	}
	_, err := Resolve(context.Background(), root, &configuration)
	if err == nil || !strings.Contains(err.Error(), "outside analysis root") {
		t.Fatalf("expected root-boundary error, got %v", err)
	}
}

func TestFactsAdapterRequiresSelectedResource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "deployment.yaml")
	writeFile(t, path, "version: 1\nresources: {}\n")
	_, err := resolveFacts(deploymentRequest(root, path, "missing"))
	if err == nil || !strings.Contains(err.Error(), `resource "missing" not found`) {
		t.Fatalf("expected missing-resource error, got %v", err)
	}
}

func deploymentRequest(root, source, resource string) protocol.DeploymentRequest {
	return protocol.DeploymentRequest{
		ProtocolVersion: protocol.DeploymentAdapterVersion,
		Root:            root, Source: source, Resource: resource,
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
