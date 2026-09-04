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
}

func Analyze(ctx context.Context, root string, options Options) ([]protocol.CodeFact, error) {
	directory, err := os.MkdirTemp("", "waldo-javascript-provider-")
	if err != nil {
		return nil, fmt.Errorf("create temporary rules directory: %w", err)
	}
	defer os.RemoveAll(directory)

	configuration := filepath.Join(directory, "semgrep.yaml")
	if err := os.WriteFile(configuration, rules, 0o600); err != nil {
		return nil, fmt.Errorf("write embedded JavaScript rules: %w", err)
	}
	return semgrepprovider.Analyze(ctx, root, semgrepprovider.Options{
		Executable: options.SemgrepExecutable,
		Configs:    []string{configuration},
		Targets:    options.Targets,
	})
}
