// Package semgrep adapts explicitly annotated Semgrep results to Waldo's
// analyzer-neutral code-fact protocol. Source-language rules stay in the
// provider; Waldo core never imports this package.
package semgrep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirage-security/waldo/protocol"
)

type output struct {
	Results []result `json:"results"`
	Errors  []any    `json:"errors"`
}

type result struct {
	CheckID string `json:"check_id"`
	Path    string `json:"path"`
	Start   struct {
		Line int `json:"line"`
		Col  int `json:"col"`
	} `json:"start"`
	Extra struct {
		Metadata map[string]any          `json:"metadata"`
		Metavars map[string]metavariable `json:"metavars"`
		Message  string                  `json:"message"`
	} `json:"extra"`
}

type metavariable struct {
	AbstractContent string `json:"abstract_content"`
}

type Options struct {
	Executable string
	Configs    []string
	Targets    []string
}

func Analyze(ctx context.Context, root string, options Options) ([]protocol.CodeFact, error) {
	if options.Executable == "" {
		options.Executable = "semgrep"
	}
	if len(options.Configs) == 0 {
		return nil, fmt.Errorf("at least one Semgrep config is required")
	}
	if len(options.Targets) == 0 {
		options.Targets = []string{"."}
	}

	arguments := []string{"scan", "--json", "--quiet", "--metrics=off", "--disable-version-check"}
	for _, config := range options.Configs {
		arguments = append(arguments, "--config", config)
	}
	arguments = append(arguments, options.Targets...)
	command := exec.CommandContext(ctx, options.Executable, arguments...)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("semgrep scan: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return Decode(root, stdout.Bytes())
}

func Decode(root string, data []byte) ([]protocol.CodeFact, error) {
	var scan output
	if err := json.Unmarshal(data, &scan); err != nil {
		return nil, fmt.Errorf("decode Semgrep output: %w", err)
	}
	if len(scan.Errors) > 0 {
		return nil, fmt.Errorf("Semgrep reported %d scan errors", len(scan.Errors))
	}

	facts := make([]protocol.CodeFact, 0, len(scan.Results))
	seen := make(map[string]struct{})
	for _, candidate := range scan.Results {
		fact, annotated, err := factFromResult(root, candidate)
		if err != nil {
			return nil, err
		}
		if !annotated {
			continue
		}
		if _, exists := seen[fact.ID]; exists {
			return nil, fmt.Errorf("duplicate fact ID %q; refine the provider rule's symbol identity", fact.ID)
		}
		seen[fact.ID] = struct{}{}
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].ID < facts[j].ID })
	return facts, nil
}

func factFromResult(root string, candidate result) (protocol.CodeFact, bool, error) {
	raw, annotated := candidate.Extra.Metadata["waldo"]
	if !annotated {
		return protocol.CodeFact{}, false, nil
	}
	metadata, ok := raw.(map[string]any)
	if !ok {
		return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q has non-object metadata.waldo", candidate.CheckID)
	}
	factType, _ := metadata["kind"].(string)
	ruleID, _ := metadata["id"].(string)
	symbolVariable, _ := metadata["symbolMetavariable"].(string)
	symbolMessagePrefix, _ := metadata["symbolMessagePrefix"].(string)
	if strings.TrimSpace(factType) == "" || strings.TrimSpace(ruleID) == "" {
		return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q metadata.waldo requires id and kind", candidate.CheckID)
	}
	if (strings.TrimSpace(symbolVariable) == "") == (strings.TrimSpace(symbolMessagePrefix) == "") {
		return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q metadata.waldo requires exactly one of symbolMetavariable or symbolMessagePrefix", candidate.CheckID)
	}

	symbol := ""
	if symbolMessagePrefix != "" {
		if !strings.HasPrefix(candidate.Extra.Message, symbolMessagePrefix) {
			return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q message does not start with symbol prefix %q", candidate.CheckID, symbolMessagePrefix)
		}
		symbol = strings.TrimSpace(strings.TrimPrefix(candidate.Extra.Message, symbolMessagePrefix))
	} else {
		matched, exists := candidate.Extra.Metavars[symbolVariable]
		if exists {
			symbol = strings.TrimSpace(matched.AbstractContent)
		}
	}
	if symbol == "" {
		return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q did not emit a semantic symbol", candidate.CheckID)
	}
	attributes := map[string]any{}
	if rawAttributes, exists := metadata["attributes"]; exists {
		var valid bool
		attributes, valid = rawAttributes.(map[string]any)
		if !valid {
			return protocol.CodeFact{}, false, fmt.Errorf("Semgrep rule %q metadata.waldo.attributes must be an object", candidate.CheckID)
		}
	}

	path := candidate.Path
	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return protocol.CodeFact{}, false, err
		}
		path = relative
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == ".." || strings.HasPrefix(path, "../") {
		return protocol.CodeFact{}, false, fmt.Errorf("Semgrep result path %q is outside root", candidate.Path)
	}
	return protocol.CodeFact{
		ID:         strings.Join([]string{"semgrep", ruleID, path, symbol}, ":"),
		Kind:       factType,
		Source:     protocol.SourceLocation{Path: path, Line: candidate.Start.Line, Column: candidate.Start.Col},
		Symbol:     symbol,
		Attributes: attributes,
	}, true, nil
}
