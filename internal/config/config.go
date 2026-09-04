package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/mirage-security/waldo/internal/model"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version      int                  `yaml:"version"`
	Deployment   Deployment           `yaml:"deployment"`
	Providers    []Provider           `yaml:"providers,omitempty"`
	Policies     []Policy             `yaml:"policies"`
	Dispositions []FindingDisposition `yaml:"dispositions,omitempty"`
}

type Deployment struct {
	Units map[string]DeploymentUnit `yaml:"units"`
}

type DeploymentUnit struct {
	CodeRoots []string       `yaml:"codeRoots"`
	Facts     map[string]any `yaml:"facts"`
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

	config, err := Decode(file)
	if err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return config, nil
}

func Decode(reader io.Reader) (Config, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Config{}, err
		}
		return Config{}, fmt.Errorf("configuration must contain exactly one YAML document")
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
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
		if len(unit.CodeRoots) == 0 {
			return fmt.Errorf("deployment unit %q must declare codeRoots", name)
		}
		for _, root := range unit.CodeRoots {
			cleaned := path.Clean(strings.ReplaceAll(root, "\\", "/"))
			if strings.TrimSpace(root) == "" || strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
				return fmt.Errorf("deployment unit %q codeRoot %q must be root-relative", name, root)
			}
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
