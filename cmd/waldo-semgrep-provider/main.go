package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mirage-security/waldo/protocol"
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

	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var request protocol.Request
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode provider request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode provider request: %w", err)
		}
		return fmt.Errorf("provider request must contain one JSON object")
	}
	if request.ProtocolVersion != protocol.Version {
		return fmt.Errorf("protocolVersion must be %d", protocol.Version)
	}
	if request.Root == "" || !filepath.IsAbs(request.Root) {
		return fmt.Errorf("root must be an absolute path")
	}

	facts, err := semgrepprovider.Analyze(ctx, request.Root, semgrepprovider.Options{
		Executable: executable,
		Configs:    configs,
		Targets:    targets,
	})
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	for _, fact := range facts {
		if err := encoder.Encode(fact); err != nil {
			return fmt.Errorf("encode fact: %w", err)
		}
	}
	return nil
}
