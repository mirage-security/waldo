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
