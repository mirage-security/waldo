package semgrep

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDecodeAnnotatedResult(t *testing.T) {
	data := []byte(`{"results":[{"check_id":"path.prefix.rule","path":"src/state.ts","start":{"line":12,"col":1},"extra":{"metadata":{"waldo":{"id":"typescript.state-handoff","kind":"coordination","symbolMetavariable":"$STATE","attributes":{"coordination.authority":"process-local"}}},"metavars":{"$STATE":{"abstract_content":"active"}}}}],"errors":[]}`)
	facts, err := Decode("/repo", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1", len(facts))
	}
	fact := facts[0]
	if fact.ID != "semgrep:typescript.state-handoff:src/state.ts:active" || fact.Kind != "coordination" || fact.Symbol != "active" {
		t.Fatalf("unexpected fact: %#v", fact)
	}
	if fact.Attributes["coordination.authority"] != "process-local" {
		t.Fatalf("attributes were not preserved: %#v", fact.Attributes)
	}

	moved := []byte(`{"results":[{"check_id":"another.path.prefix.rule","path":"src/state.ts","start":{"line":99,"col":7},"extra":{"metadata":{"waldo":{"id":"typescript.state-handoff","kind":"coordination","symbolMetavariable":"$STATE","attributes":{"coordination.authority":"process-local"}}},"metavars":{"$STATE":{"abstract_content":"active"}}}}],"errors":[]}`)
	movedFacts, err := Decode("/repo", moved)
	if err != nil {
		t.Fatal(err)
	}
	if movedFacts[0].ID != fact.ID {
		t.Fatalf("line movement or Semgrep config path changed identity: %q != %q", movedFacts[0].ID, fact.ID)
	}
}

func TestDecodeAnnotatedResultFromMessage(t *testing.T) {
	data := []byte(`{"results":[{"check_id":"path.prefix.rule","path":"src/state.ts","start":{"line":12,"col":1},"extra":{"message":"waldo-symbol:active","metadata":{"waldo":{"id":"typescript.state-handoff","kind":"coordination","symbolMessagePrefix":"waldo-symbol:","attributes":{"coordination.authority":"process-local"}}}}}],"errors":[]}`)
	facts, err := Decode("/repo", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || facts[0].Symbol != "active" {
		t.Fatalf("unexpected facts: %#v", facts)
	}
}

func TestDecodeIgnoresUnannotatedResult(t *testing.T) {
	data := []byte(`{"results":[{"check_id":"ordinary.lint","path":"src/state.ts","start":{"line":1,"col":1},"extra":{"metadata":{},"metavars":{}}}],"errors":[]}`)
	facts, err := Decode("/repo", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Fatalf("got facts from an unannotated analyzer rule: %#v", facts)
	}
}

func TestProcessLocalStateHandoffRule(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep is not installed")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := Analyze(context.Background(), root, Options{
		Configs: []string{"providers/semgrep/testdata/rule.yaml"},
		Targets: []string{"providers/semgrep/testdata"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || facts[0].Symbol != "active" {
		t.Fatalf("got %#v, want one positive fact and no negative facts", facts)
	}
}
