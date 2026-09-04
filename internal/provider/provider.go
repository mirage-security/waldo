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
)

type Request struct {
	ProtocolVersion int    `json:"protocolVersion"`
	Root            string `json:"root"`
}

func Collect(ctx context.Context, root string, providers []config.Provider) ([]model.CodeFact, error) {
	var facts []model.CodeFact
	for _, configured := range providers {
		providerFacts, err := run(ctx, root, configured)
		if err != nil {
			return nil, err
		}
		facts = append(facts, providerFacts...)
	}
	return facts, nil
}

func run(ctx context.Context, root string, provider config.Provider) ([]model.CodeFact, error) {
	request, err := json.Marshal(Request{ProtocolVersion: model.SchemaVersion, Root: root})
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, provider.Command[0], provider.Command[1:]...)
	command.Dir = root
	command.Stdin = bytes.NewReader(append(request, '\n'))
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("provider %q failed: %w: %s", provider.Name, err, strings.TrimSpace(stderr.String()))
	}

	facts, err := DecodeFacts(&stdout, provider.Name)
	if err != nil {
		return nil, fmt.Errorf("provider %q output: %w", provider.Name, err)
	}
	for index := range facts {
		facts[index].Provider = provider.Name
		if err := normalizeFactPath(root, &facts[index]); err != nil {
			return nil, fmt.Errorf("provider %q fact %q: %w", provider.Name, facts[index].ID, err)
		}
	}
	if err := validateUnique(facts); err != nil {
		return nil, fmt.Errorf("provider %q output: %w", provider.Name, err)
	}
	return facts, nil
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
		if facts[index].Provider == "" {
			facts[index].Provider = "facts"
		}
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
