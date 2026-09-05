// Package providercmd handles the shared JSON boundary for built-in code
// provider commands.
package providercmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mirage-security/waldo/protocol"
)

// Run reads one provider request, calls analyze, and writes JSONL code facts.
func Run(input io.Reader, output io.Writer, analyze func(protocol.Request) ([]protocol.CodeFact, error)) error {
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

	facts, err := analyze(request)
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
