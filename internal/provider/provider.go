package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
	"github.com/mirage-security/waldo/protocol"
)

type Collection struct {
	Facts []model.CodeFact
	Runs  []model.ProviderRun
}

func Collect(ctx context.Context, root string, providers []config.Provider) (Collection, error) {
	var facts []model.CodeFact
	runs := make([]model.ProviderRun, 0, len(providers))
	for _, configured := range providers {
		result, err := run(ctx, root, configured)
		if err != nil {
			return Collection{}, err
		}
		facts = append(facts, result.Facts...)
		runs = append(runs, model.ProviderRun{
			Name:                  configured.Name,
			CodeFacts:             len(result.Facts),
			Coverage:              result.Summary.Coverage,
			FilesAttempted:        result.Summary.FilesAttempted,
			FilesNotFullyAnalyzed: result.Summary.FilesNotFullyAnalyzed,
		})
	}
	return Collection{Facts: facts, Runs: runs}, nil
}

func run(ctx context.Context, root string, provider config.Provider) (protocol.ProviderResult, error) {
	version := provider.ProtocolVersion
	if version == 0 {
		version = protocol.Version
	}
	request, err := json.Marshal(protocol.Request{ProtocolVersion: version, Root: root})
	if err != nil {
		return protocol.ProviderResult{}, err
	}
	command := exec.CommandContext(ctx, provider.Command[0], provider.Command[1:]...)
	command.Dir = root
	command.Stdin = bytes.NewReader(append(request, '\n'))
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return protocol.ProviderResult{}, fmt.Errorf("provider %q failed: %w: %s", provider.Name, err, strings.TrimSpace(stderr.String()))
	}

	var result protocol.ProviderResult
	if version == protocol.Version {
		result.Facts, err = DecodeFacts(&stdout, provider.Name)
	} else {
		result, err = DecodeProviderRecords(&stdout, provider.Name)
	}
	if err != nil {
		return protocol.ProviderResult{}, fmt.Errorf("provider %q output: %w", provider.Name, err)
	}
	for index := range result.Facts {
		result.Facts[index].Provider = provider.Name
		if err := normalizeFactPath(root, &result.Facts[index]); err != nil {
			return protocol.ProviderResult{}, fmt.Errorf("provider %q fact %q: %w", provider.Name, result.Facts[index].ID, err)
		}
	}
	if err := validateUnique(result.Facts); err != nil {
		return protocol.ProviderResult{}, fmt.Errorf("provider %q output: %w", provider.Name, err)
	}
	return result, nil
}

func LoadFacts(path string, root string) ([]model.CodeFact, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	facts, err := DecodeFacts(file, "facts")
	if err != nil {
		return nil, err
	}
	for index := range facts {
		if err := normalizeFactPath(root, &facts[index]); err != nil {
			return nil, fmt.Errorf("fact %q: %w", facts[index].ID, err)
		}
	}
	if err := validateUnique(facts); err != nil {
		return nil, err
	}
	return facts, nil
}

func DecodeFacts(reader io.Reader, defaultProvider string) ([]model.CodeFact, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var facts []model.CodeFact
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var fact model.CodeFact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if fact.Provider == "" {
			fact.Provider = defaultProvider
		}
		if err := validateFact(fact); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		facts = append(facts, fact)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func DecodeProviderRecords(reader io.Reader, defaultProvider string) (protocol.ProviderResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var result protocol.ProviderResult
	line := 0
	seenSummary := false
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		if seenSummary {
			return protocol.ProviderResult{}, fmt.Errorf("line %d: provider summary must be the final record", line)
		}
		var record protocol.ProviderRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return protocol.ProviderResult{}, fmt.Errorf("line %d: %w", line, err)
		}
		switch record.Type {
		case protocol.ProviderRecordFact:
			if record.Fact == nil || record.Summary != nil {
				return protocol.ProviderResult{}, fmt.Errorf("line %d: fact record must contain only fact", line)
			}
			fact := *record.Fact
			if fact.Provider == "" {
				fact.Provider = defaultProvider
			}
			if err := validateFact(fact); err != nil {
				return protocol.ProviderResult{}, fmt.Errorf("line %d: %w", line, err)
			}
			result.Facts = append(result.Facts, fact)
		case protocol.ProviderRecordSummary:
			if record.Summary == nil || record.Fact != nil {
				return protocol.ProviderResult{}, fmt.Errorf("line %d: summary record must contain only summary", line)
			}
			if err := validateProviderSummary(*record.Summary); err != nil {
				return protocol.ProviderResult{}, fmt.Errorf("line %d: %w", line, err)
			}
			result.Summary = *record.Summary
			seenSummary = true
		default:
			return protocol.ProviderResult{}, fmt.Errorf("line %d: unknown provider record type %q", line, record.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return protocol.ProviderResult{}, err
	}
	if !seenSummary {
		return protocol.ProviderResult{}, fmt.Errorf("provider stream is missing its summary record")
	}
	return result, nil
}

func validateProviderSummary(summary protocol.ProviderSummary) error {
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

func validateFact(fact model.CodeFact) error {
	if strings.TrimSpace(fact.ID) == "" {
		return fmt.Errorf("fact ID cannot be empty")
	}
	if strings.TrimSpace(fact.Kind) == "" {
		return fmt.Errorf("fact %q kind cannot be empty", fact.ID)
	}
	if strings.TrimSpace(fact.Source.Path) == "" {
		return fmt.Errorf("fact %q source.path cannot be empty", fact.ID)
	}
	if fact.Source.Line < 0 || fact.Source.Column < 0 {
		return fmt.Errorf("fact %q source coordinates cannot be negative", fact.ID)
	}
	return nil
}

func validateUnique(facts []model.CodeFact) error {
	seen := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		identity := fact.Provider + "\x00" + fact.ID
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("duplicate fact ID %q from provider %q", fact.ID, fact.Provider)
		}
		seen[identity] = struct{}{}
	}
	return nil
}

func normalizeFactPath(root string, fact *model.CodeFact) error {
	path := filepath.Clean(fact.Source.Path)
	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		path = relative
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == ".." || strings.HasPrefix(path, "../") {
		return fmt.Errorf("source path %q is outside root", fact.Source.Path)
	}
	fact.Source.Path = path
	return nil
}
