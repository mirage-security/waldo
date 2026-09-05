// Package compose extracts deployment facts from Docker Compose service definitions.
package compose

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirage-security/waldo/protocol"
	"gopkg.in/yaml.v3"
)

type document struct {
	Services map[string]service `yaml:"services"`
}

type service struct {
	ContainerName string `yaml:"container_name"`
	Scale         any    `yaml:"scale"`
	Deploy        *struct {
		Mode     string `yaml:"mode"`
		Replicas any    `yaml:"replicas"`
	} `yaml:"deploy"`
}

// Analyze selects one service from an existing Compose file without invoking Docker.
func Analyze(request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	source, err := withinRoot(request.Root, request.Source)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	if !isYAML(source) {
		return protocol.DeploymentResult{}, fmt.Errorf("source %q is not a YAML file", request.Source)
	}
	kind, name, err := parseResource(request.Resource)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	if kind != "service" {
		return protocol.DeploymentResult{}, fmt.Errorf("resource kind %q is unsupported; expected service/name", kind)
	}

	contents, err := os.ReadFile(source)
	if err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("read source: %w", err)
	}
	var configuration document
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	if err := decoder.Decode(&configuration); err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("decode %q: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return protocol.DeploymentResult{}, fmt.Errorf("decode %q: %w", source, err)
		}
		return protocol.DeploymentResult{}, fmt.Errorf("source %q must contain exactly one YAML document", request.Source)
	}
	selected, exists := configuration.Services[name]
	if !exists {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q not found in %q", request.Resource, request.Source)
	}

	facts := map[string]any{
		"platform.executionModel":         "composed-container",
		"process.restartable":             true,
		"memory.scope":                    "instance",
		"scheduling.processLocal.durable": false,
	}
	replicas, known, err := replicaCount(selected)
	if err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q: %w", request.Resource, err)
	}
	if known {
		facts["deployment.replicas.concurrent"] = replicas > 1
		facts["deployment.replicas.maxConcurrent"] = replicas
	}
	return protocol.DeploymentResult{Facts: facts}, nil
}

func replicaCount(selected service) (int, bool, error) {
	scale, hasScale := optionalNonNegativeInteger(selected.Scale)

	var deployReplicas int
	var hasDeployReplicas bool
	if selected.Deploy != nil {
		if selected.Deploy.Mode != "" && selected.Deploy.Mode != "replicated" {
			return 0, false, nil
		}
		deployReplicas, hasDeployReplicas = optionalNonNegativeInteger(selected.Deploy.Replicas)
	}
	if (selected.Scale != nil && !hasScale) ||
		(selected.Deploy != nil && selected.Deploy.Replicas != nil && !hasDeployReplicas) {
		return 0, false, nil
	}
	if hasScale && hasDeployReplicas && scale != deployReplicas {
		return 0, false, fmt.Errorf("scale and deploy.replicas must agree")
	}

	replicas := 1
	switch {
	case hasScale:
		replicas = scale
	case hasDeployReplicas:
		replicas = deployReplicas
	}
	if selected.ContainerName != "" && replicas > 1 {
		return 0, false, fmt.Errorf("container_name prevents scaling beyond one container")
	}
	return replicas, true, nil
}

func optionalNonNegativeInteger(value any) (int, bool) {
	if value == nil {
		return 0, false
	}
	switch value := value.(type) {
	case int:
		return value, value >= 0
	case int64:
		converted := int(value)
		return converted, value >= 0 && int64(converted) == value
	case uint64:
		converted := int(value)
		return converted, converted >= 0 && uint64(converted) == value
	case float64:
		converted := int(value)
		return converted, value >= 0 && value == float64(converted)
	default:
		return 0, false
	}
}

func parseResource(resource string) (string, string, error) {
	kind, name, ok := strings.Cut(resource, "/")
	if !ok || kind == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("resource must have the form service/name")
	}
	return kind, name, nil
}

func isYAML(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func withinRoot(root, path string) (string, error) {
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}

	sourcePath := path
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(resolvedRoot, sourcePath)
	}
	resolvedSource, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return "", fmt.Errorf("resolve source: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedSource)
	if err != nil {
		return "", fmt.Errorf("compare source with root: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source %q escapes root %q", path, root)
	}
	return resolvedSource, nil
}
