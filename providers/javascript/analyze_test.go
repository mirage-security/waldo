package javascript

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mirage-security/waldo/protocol"
)

func TestAssignedAsyncTimeout(t *testing.T) {
	root := javascriptTestRoot(t)

	allFacts, err := Analyze(context.Background(), root, Options{
		Targets: []string{"providers/javascript/testdata"},
	})
	if err != nil {
		t.Fatal(err)
	}
	allTimeouts := factsWithKind(allFacts, "deferred-execution")
	if len(allTimeouts) != 2 {
		t.Fatalf("got %d timeout facts without exclusions, want 2: %#v", len(allTimeouts), allFacts)
	}

	facts, err := Analyze(context.Background(), root, Options{
		Targets:  []string{"providers/javascript/testdata"},
		Excludes: []string{"**/*.test.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	timeouts := factsWithKind(facts, "deferred-execution")
	if len(timeouts) != 1 {
		t.Fatalf("got %d timeout facts, want 1: %#v", len(timeouts), facts)
	}
	fact := timeouts[0]
	if fact.Kind != "deferred-execution" || fact.Symbol != "expiryTimer" {
		t.Fatalf("unexpected fact: %#v", fact)
	}
	if fact.Attributes["execution.authority"] != "process-local" || fact.Attributes["execution.scheduler"] != "timer" {
		t.Fatalf("unexpected attributes: %#v", fact.Attributes)
	}
	if fact.Attributes["correctness.criticality"] != "unknown" {
		t.Fatalf("provider must preserve unknown architectural criticality: %#v", fact.Attributes)
	}
	if _, exists := fact.Attributes["correctness.critical"]; exists {
		t.Fatalf("language provider must not infer architectural criticality: %#v", fact.Attributes)
	}
}

func javascriptTestRoot(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("semgrep"); err != nil {
		if os.Getenv("WALDO_REQUIRE_SEMGREP") == "1" {
			t.Fatal("semgrep is required for JavaScript provider integration tests")
		}
		t.Skip("semgrep is not installed")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func factsWithKind(facts []protocol.CodeFact, kind string) []protocol.CodeFact {
	filtered := make([]protocol.CodeFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Kind == kind {
			filtered = append(filtered, fact)
		}
	}
	return filtered
}

func TestProcessLocalAuthorityFacts(t *testing.T) {
	root := javascriptTestRoot(t)
	facts, err := Analyze(context.Background(), root, Options{
		Targets: []string{"examples/process-local-authority/app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 3 {
		t.Fatalf("got %d facts, want 3: %#v", len(facts), facts)
	}
	wantSymbols := map[string]bool{
		"findCachedCatalogEntry": false,
		"loadDraft":              false,
		"readOption":             false,
	}
	for _, fact := range facts {
		if fact.Kind != "state-dependent-decision" {
			t.Fatalf("unexpected fact kind: %#v", fact)
		}
		if fact.Attributes["state.scope"] != "process-local" || fact.Attributes["state.role"] != "unknown" {
			t.Fatalf("unexpected authority attributes: %#v", fact.Attributes)
		}
		if _, exists := wantSymbols[fact.Symbol]; !exists {
			t.Fatalf("unexpected authority symbol %q: %#v", fact.Symbol, fact)
		}
		wantSymbols[fact.Symbol] = true
	}
	for symbol, seen := range wantSymbols {
		if !seen {
			t.Fatalf("missing authority fact for %q", symbol)
		}
	}
}

func TestProcessLocalCoordinationFact(t *testing.T) {
	root := javascriptTestRoot(t)
	facts, err := Analyze(context.Background(), root, Options{
		Targets: []string{"examples/process-local-coordination/app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1: %#v", len(facts), facts)
	}
	fact := facts[0]
	if fact.Kind != "coordination" || fact.Symbol != "answered" {
		t.Fatalf("unexpected coordination fact: %#v", fact)
	}
	if fact.Attributes["coordination.confidence"] != "high" || fact.Attributes["coordination.requiredScope"] != "deployment" {
		t.Fatalf("unexpected coordination attributes: %#v", fact.Attributes)
	}
}
