package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestRunRejectsInvalidProtocolVersion(t *testing.T) {
	request := protocol.DeploymentRequest{
		ProtocolVersion: 99,
		Root:            filepath.Clean(t.TempDir()),
		Source:          filepath.Clean(t.TempDir()),
		Resource:        "module.service",
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
