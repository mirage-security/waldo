package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mirage-security/waldo/internal/config"
)

func TestRunProviderBoundary(t *testing.T) {
	configured := config.Provider{
		Name:    "test-provider",
		Command: []string{os.Args[0], "-test.run=TestProviderHelper", "--", "emit"},
	}
	facts, err := run(context.Background(), t.TempDir(), configured)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || facts[0].Provider != "test-provider" || facts[0].ID != "structural:one" {
		t.Fatalf("unexpected facts: %#v", facts)
	}
}

func TestDecodeFactsRejectsInvalidRecords(t *testing.T) {
	_, err := DecodeFacts(strings.NewReader(`{"kind":"example","source":{"path":"src/a"}}`), "test")
	if err == nil || !strings.Contains(err.Error(), "ID cannot be empty") {
		t.Fatalf("expected missing ID error, got %v", err)
	}
}

func TestDecodeFactsRejectsDuplicateProviderIdentity(t *testing.T) {
	input := strings.NewReader("" +
		`{"id":"same","provider":"test","kind":"example","source":{"path":"src/a"}}` + "\n" +
		`{"id":"same","provider":"test","kind":"example","source":{"path":"src/b"}}` + "\n")
	facts, err := DecodeFacts(input, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUnique(facts); err == nil || !strings.Contains(err.Error(), "duplicate fact ID") {
		t.Fatalf("expected duplicate identity error, got %v", err)
	}
}

func TestProviderHelper(t *testing.T) {
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) && os.Args[index+1] == "emit" {
			fmt.Println(`{"id":"structural:one","kind":"example","source":{"path":"src/example"}}`)
			os.Exit(0)
		}
	}
}
