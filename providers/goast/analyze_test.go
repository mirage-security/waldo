package goast

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirage-security/waldo/protocol"
)

func TestAnalyzeDistinguishesCriticalAndBestEffortTimers(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	facts, err := Analyze(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	facts = exampleFacts(facts)
	if len(facts) != 2 {
		t.Fatalf("got %d example facts, want 2: %#v", len(facts), facts)
	}
	critical := 0
	bestEffort := 0
	for _, fact := range facts {
		if fact.Kind != "deferred-execution" || fact.Attributes["execution.authority"] != "process-local" {
			t.Fatalf("unexpected deferred-execution fact: %#v", fact)
		}
		if fact.Attributes["correctness.critical"] == true {
			critical++
		} else {
			bestEffort++
		}
	}
	if critical != 1 || bestEffort != 1 {
		t.Fatalf("got %d critical and %d best-effort facts", critical, bestEffort)
	}
}

func TestFactIdentitySurvivesLineMovement(t *testing.T) {
	root := t.TempDir()
	filename := filepath.Join(root, "expiry.go")
	before := `package expiry
import "time"
// waldo:correctness-critical-deferred-work
func Schedule(delay time.Duration, work func()) { time.AfterFunc(delay, work) }
`
	after := `package expiry

import "time"

// A line-moving documentation edit.
// waldo:correctness-critical-deferred-work
func Schedule(delay time.Duration, work func()) {
	time.AfterFunc(delay, work)
}
`
	if err := os.WriteFile(filename, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := analyzeFile(root, "example.test/expiry", filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(after), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := analyzeFile(root, "example.test/expiry", filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatalf("line movement changed fact identity: %#v then %#v", first, second)
	}
	if first[0].Source.Line == second[0].Source.Line {
		t.Fatal("test input did not move the source line")
	}
}

func exampleFacts(facts []protocol.CodeFact) []protocol.CodeFact {
	var matching []protocol.CodeFact
	for _, fact := range facts {
		if strings.HasPrefix(fact.Source.Path, "examples/durable-deferred-work/app/") {
			matching = append(matching, fact)
		}
	}
	return matching
}
