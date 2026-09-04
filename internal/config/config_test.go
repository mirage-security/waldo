package config

import (
	"strings"
	"testing"
)

func TestDecodeRejectsUnknownFields(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts: {}
policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
surprise: true
`
	_, err := Decode(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "field surprise not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestDispositionRequiresReason(t *testing.T) {
	input := `
version: 1
deployment:
  units:
    worker:
      codeRoots: [src]
      facts: {}
policies:
  - id: example
    title: Example
    severity: info
    when:
      deployment: {}
      code:
        kind: example
    message: Example.
dispositions:
  - finding: waldo:v1:0000000000000000000000000000000000000000000000000000000000000000
    disposition: accepted
    reason: ""
`
	_, err := Decode(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "must include a reason") {
		t.Fatalf("expected reason validation error, got %v", err)
	}
}
