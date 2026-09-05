package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	terraformadapter "github.com/mirage-security/waldo/adapters/terraform"
	"github.com/mirage-security/waldo/protocol"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var request protocol.DeploymentRequest
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("decode deployment request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode deployment request: %w", err)
		}
		return fmt.Errorf("deployment request must contain one JSON object")
	}
	if request.ProtocolVersion != protocol.DeploymentAdapterVersion {
		return fmt.Errorf("protocolVersion must be %d", protocol.DeploymentAdapterVersion)
	}
	if request.Root == "" || !filepath.IsAbs(request.Root) {
		return fmt.Errorf("root must be an absolute path")
	}
	if request.Source == "" || !filepath.IsAbs(request.Source) {
		return fmt.Errorf("source must be an absolute path")
	}
	if request.Resource == "" {
		return fmt.Errorf("resource cannot be empty")
	}
	result, err := terraformadapter.Analyze(request)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return fmt.Errorf("encode deployment result: %w", err)
	}
	return nil
}
