// Package deployment resolves deployment bindings through replaceable adapters.
package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
	"github.com/mirage-security/waldo/protocol"
	"gopkg.in/yaml.v3"
)

const factsAdapter = "facts"

type factsDocument struct {
	Version   int                                  `yaml:"version"`
	Resources map[string]protocol.DeploymentResult `yaml:"resources"`
}

// Resolve validates artifact roots and fills every deployment's normalized
// facts by invoking the adapter named in its from binding.
func Resolve(ctx context.Context, root string, configuration *config.Config) ([]model.DeploymentAdapterRun, error) {
	if configuration == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve analysis root: %w", err)
	}
	baseDir := configuration.BaseDir
	if baseDir == "" {
		baseDir = root
	}
	for name, artifact := range configuration.Artifacts {
		source := artifact.Source
		if source == "" {
			source = "."
		}
		absoluteSource := filepath.Join(baseDir, filepath.FromSlash(source))
		relativeSource, err := pathWithinRoot(root, absoluteSource)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: %w", name, err)
		}
		artifact.ResolvedSource = relativeSource
		configuration.Artifacts[name] = artifact
	}

	names := make([]string, 0, len(configuration.Deployments))
	for name := range configuration.Deployments {
		names = append(names, name)
	}
	sort.Strings(names)
	runs := make([]model.DeploymentAdapterRun, 0, len(names))
	for _, name := range names {
		configured := configuration.Deployments[name]
		source := filepath.Join(baseDir, filepath.FromSlash(configured.From.Source))
		if _, err := pathWithinRoot(root, source); err != nil {
			return nil, fmt.Errorf("deployment %q: %w", name, err)
		}
		request := protocol.DeploymentRequest{
			ProtocolVersion: protocol.DeploymentAdapterVersion,
			Root:            root,
			Source:          filepath.Clean(source),
			Resource:        configured.From.Resource,
			Options:         configured.From.With,
		}
		result, err := resolveOne(ctx, baseDir, configured.From.Adapter, request)
		if err != nil {
			return nil, fmt.Errorf("deployment %q adapter %q: %w", name, configured.From.Adapter, err)
		}
		if err := validateFacts(result.Facts); err != nil {
			return nil, fmt.Errorf("deployment %q adapter %q: %w", name, configured.From.Adapter, err)
		}
		configured.Facts = result.Facts
		configuration.Deployments[name] = configured
		runs = append(runs, model.DeploymentAdapterRun{
			Deployment: identity(configuration.Service, name),
			Adapter:    configured.From.Adapter,
			Facts:      len(result.Facts),
		})
	}
	return runs, nil
}

func identity(service, deployment string) string {
	return service + "/" + deployment
}

func resolveOne(ctx context.Context, baseDir, adapter string, request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	if adapter == factsAdapter {
		return resolveFacts(request)
	}
	commandName := adapter
	if !strings.ContainsAny(adapter, `/\\`) {
		commandName = "waldo-" + adapter + "-deployment-adapter"
	} else if !filepath.IsAbs(adapter) {
		commandName = filepath.Join(baseDir, filepath.FromSlash(adapter))
	}
	return runExternal(ctx, baseDir, commandName, request)
}

func resolveFacts(request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	file, err := os.Open(request.Source)
	if err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("open source: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var document factsDocument
	if err := decoder.Decode(&document); err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("decode source: %w", err)
	}
	if err := requireYAMLEOF(decoder); err != nil {
		return protocol.DeploymentResult{}, err
	}
	if document.Version != protocol.DeploymentAdapterVersion {
		return protocol.DeploymentResult{}, fmt.Errorf("source version must be %d", protocol.DeploymentAdapterVersion)
	}
	result, ok := document.Resources[request.Resource]
	if !ok {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q not found", request.Resource)
	}
	return result, nil
}

func runExternal(ctx context.Context, baseDir, commandName string, request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	command := exec.CommandContext(ctx, commandName)
	command.Dir = baseDir
	command.Stdin = bytes.NewReader(append(payload, '\n'))
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	var result protocol.DeploymentResult
	if err := decoder.Decode(&result); err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("decode output: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return protocol.DeploymentResult{}, err
	}
	return result, nil
}

func validateFacts(facts map[string]any) error {
	if facts == nil {
		return fmt.Errorf("facts cannot be null")
	}
	for name := range facts {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("fact name cannot be empty")
		}
	}
	return nil
}

func pathWithinRoot(root, candidate string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve analysis root: %w", err)
	}
	canonicalCandidate, err := filepath.EvalSymlinks(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("resolve source: %w", err)
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalCandidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source %q is outside analysis root", candidate)
	}
	return filepath.ToSlash(filepath.Clean(relative)), nil
}

func requireYAMLEOF(decoder *yaml.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("source must contain exactly one YAML document")
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("adapter output must contain exactly one JSON object")
}
