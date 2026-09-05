// Package javascript extracts analyzer-neutral facts from JavaScript and
// TypeScript. Its current backend is Semgrep, which remains an implementation
// detail behind the provider protocol.
package javascript

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mirage-security/waldo/protocol"
	semgrepprovider "github.com/mirage-security/waldo/providers/semgrep"
)

//go:embed semgrep.yaml
var rules []byte

type Options struct {
	SemgrepExecutable string
	Targets           []string
	Excludes          []string
}

func Analyze(ctx context.Context, root string, options Options) ([]protocol.CodeFact, error) {
	configuration, cleanup, err := writeRules()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return semgrepprovider.Analyze(ctx, root, semgrepOptions(configuration, options))
}

// AnalyzeDetailed preserves warning-level partial parsing as explicit provider
// coverage while retaining facts from successfully analyzed source.
func AnalyzeDetailed(ctx context.Context, root string, options Options) (protocol.ProviderResult, error) {
	configuration, cleanup, err := writeRules()
	if err != nil {
		return protocol.ProviderResult{}, err
	}
	defer cleanup()

	return semgrepprovider.AnalyzeDetailed(ctx, root, semgrepOptions(configuration, options))
}

func writeRules() (string, func(), error) {
	directory, err := os.MkdirTemp("", "waldo-javascript-provider-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temporary rules directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(directory) }

	configuration := filepath.Join(directory, "semgrep.yaml")
	if err := os.WriteFile(configuration, rules, 0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write embedded JavaScript rules: %w", err)
	}
	return configuration, cleanup, nil
}

func semgrepOptions(configuration string, options Options) semgrepprovider.Options {
	return semgrepprovider.Options{
		Executable: options.SemgrepExecutable,
		Configs:    []string{configuration},
		Targets:    options.Targets,
		Excludes:   options.Excludes,
	}
}
