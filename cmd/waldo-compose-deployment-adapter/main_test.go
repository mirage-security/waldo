package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestRunReturnsServiceFacts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(source, []byte("services:\n  app:\n    image: example/app\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := protocol.DeploymentRequest{
		ProtocolVersion: protocol.DeploymentAdapterVersion,
		Root:            root,
		Source:          source,
		Resource:        "service/app",
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
	if result.Facts["process.restartable"] != true || result.Facts["deployment.replicas.concurrent"] != false {
		t.Fatalf("unexpected result: %#v", result.Facts)
	}
}
