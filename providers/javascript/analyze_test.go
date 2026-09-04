package javascript

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAssignedAsyncTimeout(t *testing.T) {
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
	facts, err := Analyze(context.Background(), root, Options{
		Targets: []string{"providers/javascript/testdata"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("got %d facts, want 1: %#v", len(facts), facts)
	}
	fact := facts[0]
	if fact.Kind != "deferred-execution" || fact.Symbol != "expiryTimer" {
		t.Fatalf("unexpected fact: %#v", fact)
	}
	if fact.Attributes["execution.authority"] != "process-local" || fact.Attributes["execution.scheduler"] != "timer" {
		t.Fatalf("unexpected attributes: %#v", fact.Attributes)
	}
	if _, exists := fact.Attributes["correctness.critical"]; exists {
		t.Fatalf("language provider must not infer architectural criticality: %#v", fact.Attributes)
	}
}
