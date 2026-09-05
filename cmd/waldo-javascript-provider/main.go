package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mirage-security/waldo/protocol"
	javascriptprovider "github.com/mirage-security/waldo/providers/javascript"
	"github.com/mirage-security/waldo/providers/providercmd"
)

type values []string

var defaultExcludes = []string{
	"**/*.test.js",
	"**/*.test.jsx",
	"**/*.test.ts",
	"**/*.test.tsx",
	"**/*.spec.js",
	"**/*.spec.jsx",
	"**/*.spec.ts",
	"**/*.spec.tsx",
}

func (v *values) String() string { return fmt.Sprint([]string(*v)) }
func (v *values) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("waldo-javascript-provider", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var targets values
	var excludes values
	var executable string
	flags.Var(&targets, "target", "root-relative scan target (repeatable; defaults to .)")
	flags.Var(&excludes, "exclude", "root-relative scan exclusion (repeatable)")
	flags.StringVar(&executable, "semgrep", "semgrep", "Semgrep executable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	return providercmd.RunVersioned(input, output, func(request protocol.Request) (protocol.ProviderResult, error) {
		return javascriptprovider.AnalyzeDetailed(ctx, request.Root, javascriptprovider.Options{
			SemgrepExecutable: executable,
			Targets:           targets,
			Excludes:          effectiveExcludes(excludes),
		})
	})
}

func effectiveExcludes(configured values) []string {
	if len(configured) > 0 {
		return configured
	}
	return defaultExcludes
}
