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
	javascriptprovider "github.com/mirage-security/waldo/providers/javascript"
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
	flags := flag.NewFlagSet("waldo-javascript-provider", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var targets values
	var executable string
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

	facts, err := javascriptprovider.Analyze(ctx, request.Root, javascriptprovider.Options{
		SemgrepExecutable: executable,
		Targets:           targets,
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
