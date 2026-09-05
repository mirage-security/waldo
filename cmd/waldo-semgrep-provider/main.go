package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mirage-security/waldo/protocol"
	"github.com/mirage-security/waldo/providers/providercmd"
	semgrepprovider "github.com/mirage-security/waldo/providers/semgrep"
)

type values []string

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
	flags := flag.NewFlagSet("waldo-semgrep-provider", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var configs values
	var targets values
	var executable string
	flags.Var(&configs, "config", "Semgrep rule configuration (repeatable)")
	flags.Var(&targets, "target", "root-relative scan target (repeatable; defaults to .)")
	flags.StringVar(&executable, "semgrep", "semgrep", "Semgrep executable")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	return providercmd.Run(input, output, func(request protocol.Request) ([]protocol.CodeFact, error) {
		return semgrepprovider.Analyze(ctx, request.Root, semgrepprovider.Options{
			Executable: executable,
			Configs:    configs,
			Targets:    targets,
		})
	})
}
