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
	request, err := decodeRequest(input)
	if err != nil {
		return err
	}
	if request.ProtocolVersion != protocol.Version {
		return fmt.Errorf("protocolVersion must be %d", protocol.Version)
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

// RunV2 writes typed fact records followed by one required coverage summary.
func RunV2(input io.Reader, output io.Writer, analyze func(protocol.Request) (protocol.ProviderResult, error)) error {
	request, err := decodeRequest(input)
	if err != nil {
		return err
	}
	if request.ProtocolVersion != protocol.ProviderProtocolVersion {
		return fmt.Errorf("protocolVersion must be %d", protocol.ProviderProtocolVersion)
	}

	result, err := analyze(request)
	if err != nil {
		return err
	}
	if err := validateSummary(result.Summary); err != nil {
		return err
	}
	return writeV2(output, result)
}

// RunVersioned preserves protocol-v1 compatibility for complete scans while
// using protocol v2 whenever partial coverage must be represented.
func RunVersioned(input io.Reader, output io.Writer, analyze func(protocol.Request) (protocol.ProviderResult, error)) error {
	request, err := decodeRequest(input)
	if err != nil {
		return err
	}
	if request.ProtocolVersion != protocol.Version && request.ProtocolVersion != protocol.ProviderProtocolVersion {
		return fmt.Errorf("protocolVersion must be %d or %d", protocol.Version, protocol.ProviderProtocolVersion)
	}

	result, err := analyze(request)
	if err != nil {
		return err
	}
	if err := validateSummary(result.Summary); err != nil {
		return err
	}
	if request.ProtocolVersion == protocol.Version {
		if result.Summary.Coverage == protocol.ProviderCoveragePartial {
			return fmt.Errorf("protocolVersion %d cannot represent partial provider coverage", protocol.Version)
		}
		encoder := json.NewEncoder(output)
		for _, fact := range result.Facts {
			if err := encoder.Encode(fact); err != nil {
				return fmt.Errorf("encode fact: %w", err)
			}
		}
		return nil
	}
	return writeV2(output, result)
}

func writeV2(output io.Writer, result protocol.ProviderResult) error {
	encoder := json.NewEncoder(output)
	for index := range result.Facts {
		record := protocol.ProviderRecord{Type: protocol.ProviderRecordFact, Fact: &result.Facts[index]}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("encode fact: %w", err)
		}
	}
	if err := encoder.Encode(protocol.ProviderRecord{Type: protocol.ProviderRecordSummary, Summary: &result.Summary}); err != nil {
		return fmt.Errorf("encode provider summary: %w", err)
	}
	return nil
}

func decodeRequest(input io.Reader) (protocol.Request, error) {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var request protocol.Request
	if err := decoder.Decode(&request); err != nil {
		return protocol.Request{}, fmt.Errorf("decode provider request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return protocol.Request{}, fmt.Errorf("decode provider request: %w", err)
		}
		return protocol.Request{}, fmt.Errorf("provider request must contain one JSON object")
	}
	if request.Root == "" || !filepath.IsAbs(request.Root) {
		return protocol.Request{}, fmt.Errorf("root must be an absolute path")
	}
	return request, nil
}

func validateSummary(summary protocol.ProviderSummary) error {
	if !summary.Coverage.Valid() {
		return fmt.Errorf("provider summary has invalid coverage %q", summary.Coverage)
	}
	if summary.FilesAttempted < 0 || summary.FilesNotFullyAnalyzed < 0 ||
		(summary.FilesAttempted > 0 && summary.FilesNotFullyAnalyzed > summary.FilesAttempted) {
		return fmt.Errorf("provider summary has invalid file counts")
	}
	if summary.Coverage == protocol.ProviderCoverageComplete && summary.FilesNotFullyAnalyzed != 0 {
		return fmt.Errorf("complete provider summary cannot contain files that were not fully analyzed")
	}
	return nil
}
