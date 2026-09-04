package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mirage-security/waldo/protocol"
	"github.com/mirage-security/waldo/providers/goast"
)

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, input io.Reader, output io.Writer) error {
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
	facts, err := goast.Analyze(ctx, request.Root)
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
