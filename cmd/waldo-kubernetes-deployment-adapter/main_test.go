package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestRunReturnsDeploymentFacts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "deployment.yaml")
	contents := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: worker\nspec:\n  replicas: 2\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	request := protocol.DeploymentRequest{
		ProtocolVersion: protocol.DeploymentAdapterVersion,
		Root:            root,
		Source:          source,
		Resource:        "Deployment/worker",
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(bytes.NewReader(payload), &output); err != nil {
		t.Fatal(err)
	}
	var result protocol.DeploymentResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Facts["deployment.replicas.concurrent"] != true {
		t.Fatalf("unexpected result: %#v", result.Facts)
	}
	if _, exists := result.Facts["deployment.replicas.maxConcurrent"]; exists {
		t.Fatalf("rolling update received a guessed maximum: %#v", result.Facts)
	}
}

func TestRunRejectsInvalidProtocolVersion(t *testing.T) {
	request := protocol.DeploymentRequest{
		ProtocolVersion: 99,
		Root:            filepath.Clean(t.TempDir()),
		Source:          filepath.Clean(t.TempDir()),
		Resource:        "Deployment/worker",
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	err = run(bytes.NewReader(payload), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "protocolVersion") {
		t.Fatalf("expected protocol error, got %v", err)
	}
}
