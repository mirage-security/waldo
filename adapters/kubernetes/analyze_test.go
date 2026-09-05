package kubernetes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestAnalyzeDeploymentDefaultsIncludeRolloutOverlap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  template: {}
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "deployment.yaml"), Resource: "Deployment/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["platform.executionModel"] != "orchestrated-container" ||
		result.Facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("unexpected facts: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.maxConcurrent"]; exists {
		t.Fatalf("rolling update received a guessed maximum: %#v", result.Facts)
	}
}

func TestAnalyzeRecreateSingleReplicaHasNoOverlap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 1
  strategy:
    type: Recreate
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "deployment.yaml"), Resource: "Deployment/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != false ||
		result.Facts["deployment.replicas.maxConcurrent"] != 1 {
		t.Fatalf("unexpected concurrency facts: %#v", result.Facts)
	}
}

func TestAnalyzeRollingUpdateOmitsNumericMaximum(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 50%
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "deployment.yaml"), Resource: "Deployment/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("rolling update did not establish concurrency: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.maxConcurrent"]; exists {
		t.Fatalf("rolling update received a guessed maximum: %#v", result.Facts)
	}
}

func TestAnalyzeExplicitZeroSurgeStillAllowsTerminatingOverlap(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 0
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "deployment.yaml"), Resource: "Deployment/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("terminating-pod overlap was missed: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.maxConcurrent"]; exists {
		t.Fatalf("rolling update received a guessed maximum: %#v", result.Facts)
	}
}

func TestAnalyzeDirectoryUsesNamespaceToDisambiguate(t *testing.T) {
	root := t.TempDir()
	manifests := filepath.Join(root, "rendered")
	if err := os.MkdirAll(manifests, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(manifests, "one.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: one
spec:
  replicas: 1
  strategy:
    type: Recreate
`)
	write(t, filepath.Join(manifests, "two.yml"), `
apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: worker
      namespace: two
    spec:
      replicas: 4
      strategy:
        type: Recreate
`)

	_, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: manifests, Resource: "Deployment/worker",
	})
	if err == nil || !strings.Contains(err.Error(), "with.namespace") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: manifests, Resource: "Deployment/worker",
		Options: map[string]any{"namespace": "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.maxConcurrent"] != 4 {
		t.Fatalf("wrong namespaced resource selected: %#v", result.Facts)
	}
}

func TestAnalyzeUnknownReplicaCountOmitsConcurrency(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "deployment.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: ${REPLICAS}
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "deployment.yaml"), Resource: "Deployment/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Facts["platform.executionModel"] != "orchestrated-container" {
		t.Fatalf("known base facts were lost: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.concurrent"]; exists {
		t.Fatalf("unknown replica count produced guessed concurrency: %#v", result.Facts)
	}
}

func TestAnalyzeUnsupportedResourceReturnsAuditableZeroFacts(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "service.yaml"), `
apiVersion: v1
kind: Service
metadata:
  name: worker
`)

	result, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: filepath.Join(root, "service.yaml"), Resource: "Service/worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Facts) != 0 {
		t.Fatalf("unsupported resource produced guessed facts: %#v", result.Facts)
	}
}

func TestAnalyzeRejectsSourceOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "deployment.yaml")
	write(t, outside, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: worker\n")

	_, err := Analyze(protocol.DeploymentRequest{
		Root: root, Source: outside, Resource: "Deployment/worker",
	})
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("expected outside-root error, got %v", err)
	}
}

func write(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
