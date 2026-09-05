package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mirage-security/waldo/internal/config"
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

func TestCheckFailsClosedWhenBuiltInProviderIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{
		"check",
		"--root", repositoryRoot,
		"--config", "testdata/waldo.yaml",
	}, &stdout, &stderr)
	if exitCode != 2 || !bytes.Contains(stderr.Bytes(), []byte(`provider "javascript" failed`)) {
		t.Fatalf("got exit %d and stderr %q", exitCode, stderr.String())
	}
}

func TestBuiltInProviderTargetsDeploymentCodeRoots(t *testing.T) {
	providers := builtInProviders(config.Deployment{Units: map[string]config.DeploymentUnit{
		"api":       {CodeRoots: []string{"services/shared", "services/api"}},
		"reporting": {CodeRoots: []string{"services/shared", "services/reporting"}},
	}})
	want := []string{
		"waldo-javascript-provider",
		"--target", "services/api",
		"--target", "services/reporting",
		"--target", "services/shared",
	}
	if len(providers) != 1 || providers[0].Name != "javascript" || !reflect.DeepEqual(providers[0].Command, want) {
		t.Fatalf("unexpected built-in providers: %#v", providers)
	}
}
