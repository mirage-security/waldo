package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if report.SchemaVersion != model.ReportSchemaVersion || report.Analysis.Input != model.AnalysisInputFactsFile || report.Analysis.CodeFacts != 1 {
		t.Fatalf("unexpected analysis accounting: %#v", report.Analysis)
	}
}

func TestCheckMakesZeroFindingsAuditable(t *testing.T) {
	root := t.TempDir()
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	configuration := fmt.Sprintf(`version: 1

deployment:
  units:
    worker:
      source:
        root: .
      facts:
        process.restartable: true

providers:
  - name: empty-provider
    command: [%q, -test.run=TestEmptyProviderHelper, --, empty]
`, executable)
	if err := os.WriteFile(filepath.Join(root, "waldo.yaml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"check", "--root", root, "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("got exit %d, want 0; stderr: %s", exitCode, stderr.String())
	}
	var report model.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Total != 0 || report.Analysis.CodeFacts != 0 || report.Analysis.DeploymentUnits != 1 {
		t.Fatalf("zero result is not accounted for: %#v", report)
	}
	if len(report.Analysis.ProviderRuns) != 1 || report.Analysis.ProviderRuns[0].Name != "empty-provider" || report.Analysis.ProviderRuns[0].CodeFacts != 0 {
		t.Fatalf("provider completion is not accounted for: %#v", report.Analysis.ProviderRuns)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run(context.Background(), []string{"check", "--root", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("got exit %d, want 0; stderr: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Analysis: 1 provider completed; 0 code facts; 1 deployment unit; 4 policies.") ||
		!strings.Contains(stdout.String(), "empty-provider: 0 code facts") ||
		!strings.Contains(stdout.String(), "0 findings") {
		t.Fatalf("human report does not explain zero result:\n%s", stdout.String())
	}
}

func TestEmptyProviderHelper(t *testing.T) {
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) && os.Args[index+1] == "empty" {
			os.Exit(0)
		}
	}
}

func TestReadReportAcceptsSchemaVersionOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.report.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"generatedAt":"2026-01-01T00:00:00Z","root":"/tmp/source","findings":[],"summary":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := readReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Analysis.Input != "" {
		t.Fatalf("unexpected legacy report: %#v", report)
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

func TestBuiltInProviderTargetsDistinctDeploymentSourceRoots(t *testing.T) {
	providers := builtInProviders(config.Deployment{Units: map[string]config.DeploymentUnit{
		"api-http":       {Source: config.DeploymentSource{Root: "services/api", Entrypoint: "src/http.ts"}},
		"api-worker":     {Source: config.DeploymentSource{Root: "services/api", Entrypoint: "src/worker.ts"}},
		"reporting-http": {Source: config.DeploymentSource{Root: "services/reporting", Entrypoint: "src/http.ts"}},
	}})
	want := []string{
		"waldo-javascript-provider",
		"--target", "services/api",
		"--target", "services/reporting",
	}
	if len(providers) != 1 || providers[0].Name != "javascript" || !reflect.DeepEqual(providers[0].Command, want) {
		t.Fatalf("unexpected built-in providers: %#v", providers)
	}
}
