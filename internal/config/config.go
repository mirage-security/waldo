package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mirage-security/waldo/internal/model"
	builtinpolicies "github.com/mirage-security/waldo/policies"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version      int                  `yaml:"version"`
	Deployment   Deployment           `yaml:"deployment"`
	Providers    []Provider           `yaml:"providers,omitempty"`
	PolicyFiles  []string             `yaml:"policyFiles,omitempty"`
	Policies     []Policy             `yaml:"policies,omitempty"`
	Dispositions []FindingDisposition `yaml:"dispositions,omitempty"`
}

type PolicyDocument struct {
	Version  int      `yaml:"version"`
	Policies []Policy `yaml:"policies"`
}

type Deployment struct {
	Units map[string]DeploymentUnit `yaml:"units"`
}

type DeploymentUnit struct {
	Source DeploymentSource `yaml:"source"`
	Facts  map[string]any   `yaml:"facts"`
}

type DeploymentSource struct {
	Root       string `yaml:"root"`
	Entrypoint string `yaml:"entrypoint,omitempty"`
}

type Provider struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
}

type Policy struct {
	ID       string         `yaml:"id"`
	Title    string         `yaml:"title"`
	Severity model.Severity `yaml:"severity"`
	When     Conditions     `yaml:"when"`
	Message  string         `yaml:"message"`
}

type Conditions struct {
	Deployment map[string]any `yaml:"deployment"`
	Code       CodeConditions `yaml:"code"`
}

type CodeConditions struct {
	Kind       string         `yaml:"kind"`
	Attributes map[string]any `yaml:"attributes,omitempty"`
}

type FindingDisposition struct {
	Finding     string            `yaml:"finding"`
	Disposition model.Disposition `yaml:"disposition"`
	Reason      string            `yaml:"reason"`
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	configuration, err := decodeConfig(file)
	if err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(configuration.PolicyFiles) == 0 && len(configuration.Policies) == 0 {
		if err := loadBuiltInPolicies(&configuration); err != nil {
			return Config{}, fmt.Errorf("load built-in policies: %w", err)
		}
	}
	seenFiles := make(map[string]struct{}, len(configuration.PolicyFiles))
	for _, configuredPath := range configuration.PolicyFiles {
		if strings.TrimSpace(configuredPath) == "" {
			return Config{}, fmt.Errorf("decode %s: policyFiles entries cannot be empty", path)
		}
		policyPath := configuredPath
		if !filepath.IsAbs(policyPath) {
			policyPath = filepath.Join(filepath.Dir(path), policyPath)
		}
		policyPath = filepath.Clean(policyPath)
		if _, exists := seenFiles[policyPath]; exists {
			return Config{}, fmt.Errorf("decode %s: duplicate policy file %q", path, configuredPath)
		}
		seenFiles[policyPath] = struct{}{}
		document, err := loadPolicyDocument(policyPath)
		if err != nil {
			return Config{}, fmt.Errorf("load policy file %s: %w", policyPath, err)
		}
		configuration.Policies = append(configuration.Policies, document.Policies...)
	}
	if err := configuration.Validate(); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return configuration, nil
}

func Decode(reader io.Reader) (Config, error) {
	configuration, err := decodeConfig(reader)
	if err != nil {
		return Config{}, err
	}
	if len(configuration.PolicyFiles) > 0 {
		return Config{}, fmt.Errorf("policyFiles require loading configuration from a file path")
	}
	if len(configuration.Policies) == 0 {
		if err := loadBuiltInPolicies(&configuration); err != nil {
			return Config{}, fmt.Errorf("load built-in policies: %w", err)
		}
	}
	if err := configuration.Validate(); err != nil {
		return Config{}, err
	}
	return configuration, nil
}

func decodeConfig(reader io.Reader) (Config, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var configuration Config
	if err := decoder.Decode(&configuration); err != nil {
		return Config{}, err
	}
	if err := requireYAMLEOF(decoder, "configuration"); err != nil {
		return Config{}, err
	}
	return configuration, nil
}

func loadPolicyDocument(path string) (PolicyDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return PolicyDocument{}, err
	}
	defer file.Close()
	return decodePolicyDocument(file)
}

func decodePolicyDocument(reader io.Reader) (PolicyDocument, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var document PolicyDocument
	if err := decoder.Decode(&document); err != nil {
		return PolicyDocument{}, err
	}
	if err := requireYAMLEOF(decoder, "policy file"); err != nil {
		return PolicyDocument{}, err
	}
	if document.Version != model.SchemaVersion {
		return PolicyDocument{}, fmt.Errorf("version must be %d", model.SchemaVersion)
	}
	if len(document.Policies) == 0 {
		return PolicyDocument{}, fmt.Errorf("policies must contain at least one policy")
	}
	return document, nil
}

func loadBuiltInPolicies(configuration *Config) error {
	documents, err := builtinpolicies.Documents()
	if err != nil {
		return err
	}
	for _, contents := range documents {
		document, err := decodePolicyDocument(strings.NewReader(string(contents)))
		if err != nil {
			return err
		}
		configuration.Policies = append(configuration.Policies, document.Policies...)
	}
	return nil
}

func requireYAMLEOF(decoder *yaml.Decoder, documentName string) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s must contain exactly one YAML document", documentName)
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != model.SchemaVersion {
		return fmt.Errorf("version must be %d", model.SchemaVersion)
	}
	if len(c.Deployment.Units) == 0 {
		return fmt.Errorf("deployment.units must contain at least one unit")
	}
	for name, unit := range c.Deployment.Units {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("deployment unit name cannot be empty")
		}
		root := unit.Source.Root
		cleanedRoot := path.Clean(strings.ReplaceAll(root, "\\", "/"))
		if strings.TrimSpace(root) == "" || strings.HasPrefix(cleanedRoot, "/") || cleanedRoot == ".." || strings.HasPrefix(cleanedRoot, "../") {
			return fmt.Errorf("deployment unit %q source.root %q must be root-relative", name, root)
		}
		entrypoint := unit.Source.Entrypoint
		if entrypoint == "" {
			continue
		}
		cleanedEntrypoint := path.Clean(strings.ReplaceAll(entrypoint, "\\", "/"))
		if strings.TrimSpace(entrypoint) == "" || cleanedEntrypoint == "." || strings.HasPrefix(cleanedEntrypoint, "/") || cleanedEntrypoint == ".." || strings.HasPrefix(cleanedEntrypoint, "../") {
			return fmt.Errorf("deployment unit %q source.entrypoint %q must be relative to source.root", name, entrypoint)
		}
	}

	providerNames := make(map[string]struct{}, len(c.Providers))
	for _, provider := range c.Providers {
		if provider.Name == "" {
			return fmt.Errorf("provider name cannot be empty")
		}
		if _, exists := providerNames[provider.Name]; exists {
			return fmt.Errorf("duplicate provider %q", provider.Name)
		}
		providerNames[provider.Name] = struct{}{}
		if len(provider.Command) == 0 || provider.Command[0] == "" {
			return fmt.Errorf("provider %q must declare a command", provider.Name)
		}
	}

	if len(c.Policies) == 0 {
		return fmt.Errorf("policies must contain at least one policy")
	}
	policyIDs := make(map[string]struct{}, len(c.Policies))
	for _, policy := range c.Policies {
		if policy.ID == "" {
			return fmt.Errorf("policy ID cannot be empty")
		}
		if strings.TrimSpace(policy.Title) == "" {
			return fmt.Errorf("policy %q must declare a title", policy.ID)
		}
		if _, exists := policyIDs[policy.ID]; exists {
			return fmt.Errorf("duplicate policy ID %q", policy.ID)
		}
		policyIDs[policy.ID] = struct{}{}
		if !policy.Severity.Valid() {
			return fmt.Errorf("policy %q has invalid severity %q", policy.ID, policy.Severity)
		}
		if policy.When.Code.Kind == "" {
			return fmt.Errorf("policy %q must match a code fact kind", policy.ID)
		}
		if policy.Message == "" {
			return fmt.Errorf("policy %q must declare a message", policy.ID)
		}
	}

	dispositionIDs := make(map[string]struct{}, len(c.Dispositions))
	for _, disposition := range c.Dispositions {
		if disposition.Finding == "" {
			return fmt.Errorf("disposition finding cannot be empty")
		}
		const prefix = "waldo:v1:"
		digest := strings.TrimPrefix(disposition.Finding, prefix)
		_, digestError := hex.DecodeString(digest)
		if !strings.HasPrefix(disposition.Finding, prefix) || len(digest) != sha256HexLength || digestError != nil {
			return fmt.Errorf("disposition finding %q is not a version 1 finding identity", disposition.Finding)
		}
		if _, exists := dispositionIDs[disposition.Finding]; exists {
			return fmt.Errorf("duplicate disposition for finding %q", disposition.Finding)
		}
		dispositionIDs[disposition.Finding] = struct{}{}
		if disposition.Disposition != model.DispositionAccepted && disposition.Disposition != model.DispositionFalsePositive {
			return fmt.Errorf("finding %q disposition must be accepted or false-positive", disposition.Finding)
		}
		if strings.TrimSpace(disposition.Reason) == "" {
			return fmt.Errorf("finding %q disposition must include a reason", disposition.Finding)
		}
	}
	return nil
}

const sha256HexLength = 64
