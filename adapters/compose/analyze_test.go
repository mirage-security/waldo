package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestAnalyzeDefaultService(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  app:
    image: example/app
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/app"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["platform.executionModel"] != "composed-container" ||
		result.Facts["process.restartable"] != true ||
		result.Facts["deployment.replicas.concurrent"] != false ||
		result.Facts["deployment.replicas.maxConcurrent"] != 1 {
		t.Fatalf("unexpected facts: %#v", result.Facts)
	}
}

func TestAnalyzeScaledService(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  worker:
    image: example/worker
    scale: 3
    deploy:
      replicas: 3
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/worker"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true ||
		result.Facts["deployment.replicas.maxConcurrent"] != 3 {
		t.Fatalf("unexpected facts: %#v", result.Facts)
	}
}

func TestAnalyzeUnknownScaleOmitsConcurrency(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  worker:
    image: example/worker
    scale: ${WORKER_REPLICAS}
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/worker"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["process.restartable"] != true {
		t.Fatalf("known base facts were lost: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.concurrent"]; exists {
		t.Fatalf("unknown scale produced guessed concurrency: %#v", result.Facts)
	}
}

func TestAnalyzeRejectsInconsistentReplicaSettings(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  worker:
    image: example/worker
    scale: 2
    deploy:
      replicas: 3
`)
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/worker"})
	if err == nil || !strings.Contains(err.Error(), "must agree") {
		t.Fatalf("expected inconsistent replica error, got %v", err)
	}
}

func TestAnalyzeRejectsScaledContainerName(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  worker:
    image: example/worker
    container_name: fixed
    scale: 2
`)
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/worker"})
	if err == nil || !strings.Contains(err.Error(), "container_name") {
		t.Fatalf("expected container name scaling error, got %v", err)
	}
}

func TestAnalyzeUnsupportedDeployModeOmitsConcurrency(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  agent:
    image: example/agent
    deploy:
      mode: global
`)
	result, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/agent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result.Facts["deployment.replicas.concurrent"]; exists {
		t.Fatalf("global mode produced guessed concurrency: %#v", result.Facts)
	}
}

func TestAnalyzeRejectsMultipleDocuments(t *testing.T) {
	root := t.TempDir()
	source := writeCompose(t, root, `
services:
  app:
    image: example/app
---
services:
  app:
    scale: 2
`)
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: source, Resource: "service/app"})
	if err == nil || !strings.Contains(err.Error(), "exactly one YAML document") {
		t.Fatalf("expected multiple-document error, got %v", err)
	}
}

func TestAnalyzeRejectsSourceOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := writeCompose(t, parent, "services:\n  app:\n    image: example/app\n")
	_, err := Analyze(protocol.DeploymentRequest{Root: root, Source: outside, Resource: "service/app"})
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("expected outside-root error, got %v", err)
	}
}

func writeCompose(t *testing.T, root, contents string) string {
	t.Helper()
	path := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
