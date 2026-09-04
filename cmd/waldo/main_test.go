package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mirage-security/waldo/internal/model"
)

func TestCheckJSONAndExitPolicy(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{
		"check",
		"--root", repositoryRoot,
		"--config", "testdata/waldo.yaml",
		"--facts", "testdata/scenarios/durable-positive/facts.jsonl",
		"--json",
	}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("got exit %d, want 1; stderr: %s", exitCode, stderr.String())
	}
	var report model.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Failing != 1 || len(report.Findings) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestCheckFailsClosedWithoutFactSource(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{
		"check",
		"--root", repositoryRoot,
		"--config", "testdata/waldo.yaml",
	}, &stdout, &stderr)
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte("no code-fact source")) {
		t.Fatalf("got exit %d and stderr %q", exitCode, stderr.String())
	}
}
