package providercmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestRun(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, err := json.Marshal(protocol.Request{ProtocolVersion: protocol.Version, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = Run(bytes.NewReader(payload), &output, func(request protocol.Request) ([]protocol.CodeFact, error) {
		if request.Root != root {
			t.Fatalf("got root %q, want %q", request.Root, root)
		}
		return []protocol.CodeFact{{ID: "one", Kind: "example", Source: protocol.SourceLocation{Path: "main.go"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(output.String()); got != `{"id":"one","kind":"example","source":{"path":"main.go"}}` {
		t.Fatalf("unexpected output %s", got)
	}
}

func TestRunV2WritesFactsAndFinalSummary(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, err := json.Marshal(protocol.Request{ProtocolVersion: protocol.ProviderProtocolVersion, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = RunV2(bytes.NewReader(payload), &output, func(request protocol.Request) (protocol.ProviderResult, error) {
		return protocol.ProviderResult{
			Facts: []protocol.CodeFact{{ID: "one", Kind: "example", Source: protocol.SourceLocation{Path: "main.go"}}},
			Summary: protocol.ProviderSummary{
				Coverage:              protocol.ProviderCoveragePartial,
				FilesAttempted:        12,
				FilesNotFullyAnalyzed: 2,
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"type":"fact"`) || !strings.Contains(lines[1], `"type":"summary"`) {
		t.Fatalf("unexpected protocol-v2 output: %s", output.String())
	}
}

func TestRunV2RejectsInvalidSummary(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, err := json.Marshal(protocol.Request{ProtocolVersion: protocol.ProviderProtocolVersion, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	err = RunV2(bytes.NewReader(payload), &bytes.Buffer{}, func(protocol.Request) (protocol.ProviderResult, error) {
		return protocol.ProviderResult{Summary: protocol.ProviderSummary{Coverage: protocol.ProviderCoverageComplete, FilesNotFullyAnalyzed: 1}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "complete") {
		t.Fatalf("expected summary error, got %v", err)
	}
}

func TestRunVersionedKeepsV1CompatibilityForCompleteCoverage(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, err := json.Marshal(protocol.Request{ProtocolVersion: protocol.Version, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = RunVersioned(bytes.NewReader(payload), &output, func(protocol.Request) (protocol.ProviderResult, error) {
		return protocol.ProviderResult{
			Facts:   []protocol.CodeFact{{ID: "one", Kind: "example", Source: protocol.SourceLocation{Path: "main.go"}}},
			Summary: protocol.ProviderSummary{Coverage: protocol.ProviderCoverageComplete},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(output.String()); got != `{"id":"one","kind":"example","source":{"path":"main.go"}}` {
		t.Fatalf("unexpected protocol-v1 output: %s", got)
	}
}

func TestRunVersionedRejectsPartialCoverageForV1(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	payload, err := json.Marshal(protocol.Request{ProtocolVersion: protocol.Version, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	err = RunVersioned(bytes.NewReader(payload), &bytes.Buffer{}, func(protocol.Request) (protocol.ProviderResult, error) {
		return protocol.ProviderResult{Summary: protocol.ProviderSummary{Coverage: protocol.ProviderCoveragePartial}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "cannot represent partial") {
		t.Fatalf("expected protocol compatibility error, got %v", err)
	}
}

func TestRunRejectsInvalidRequest(t *testing.T) {
	tests := map[string]string{
		`{"protocolVersion":99,"root":"/tmp"}`:                                    "protocolVersion",
		`{"protocolVersion":1,"root":"relative"}`:                                 "absolute path",
		`{"protocolVersion":1,"root":"/tmp"} {"protocolVersion":1,"root":"/tmp"}`: "one JSON object",
	}
	for input, want := range tests {
		err := Run(strings.NewReader(input), &bytes.Buffer{}, func(protocol.Request) ([]protocol.CodeFact, error) {
			t.Fatal("analyze called for invalid request")
			return nil, nil
		})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(%q) error = %v, want %q", input, err, want)
		}
	}
}
