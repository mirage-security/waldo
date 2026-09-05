// Package kubernetes extracts deployment facts from rendered Kubernetes manifests.
package kubernetes

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mirage-security/waldo/protocol"
	"gopkg.in/yaml.v3"
)

const (
	deploymentAPIVersion = "apps/v1"
	deploymentKind       = "Deployment"
)

type manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec map[string]any `yaml:"spec"`

	Items []manifest `yaml:"items"`
}

// Analyze handles a deployment adapter request without invoking Kubernetes tooling.
func Analyze(request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	source, err := withinRoot(request.Root, request.Source)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}

	kind, name, err := parseResource(request.Resource)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	namespace, err := optionString(request.Options, "namespace")
	if err != nil {
		return protocol.DeploymentResult{}, err
	}

	paths, err := manifestPaths(request.Root, source)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}

	var matches []manifest
	for _, path := range paths {
		manifests, err := decodeFile(path)
		if err != nil {
			return protocol.DeploymentResult{}, fmt.Errorf("decode %q: %w", path, err)
		}
		for _, candidate := range manifests {
			if candidate.Kind != kind || candidate.Metadata.Name != name {
				continue
			}
			if namespace != "" && candidate.Metadata.Namespace != namespace {
				continue
			}
			matches = append(matches, candidate)
		}
	}

	if len(matches) == 0 {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q not found in %q", request.Resource, request.Source)
	}
	if len(matches) > 1 {
		if namespace == "" {
			return protocol.DeploymentResult{}, fmt.Errorf("resource %q is ambiguous; set with.namespace", request.Resource)
		}
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q in namespace %q is ambiguous", request.Resource, namespace)
	}

	return protocol.DeploymentResult{Facts: factsFor(matches[0])}, nil
}

func factsFor(value manifest) map[string]any {
	if value.APIVersion != deploymentAPIVersion || value.Kind != deploymentKind {
		return map[string]any{}
	}

	facts := map[string]any{
		"platform.executionModel":         "orchestrated-container",
		"process.restartable":             true,
		"memory.scope":                    "instance",
		"scheduling.processLocal.durable": false,
	}

	replicas, ok := replicas(value.Spec)
	if !ok {
		return facts
	}
	concurrent, maximum, hasMaximum, ok := concurrency(value.Spec, replicas)
	if !ok {
		return facts
	}
	facts["deployment.replicas.concurrent"] = concurrent
	if hasMaximum {
		facts["deployment.replicas.maxConcurrent"] = maximum
	}
	return facts
}

func replicas(spec map[string]any) (int, bool) {
	value, exists := spec["replicas"]
	if !exists || value == nil {
		return 1, true
	}
	return nonNegativeInteger(value)
}

func concurrency(spec map[string]any, replicas int) (bool, int, bool, bool) {
	strategy, exists := spec["strategy"]
	if !exists || strategy == nil {
		return replicas > 0, 0, false, true
	}

	strategyMap, ok := strategy.(map[string]any)
	if !ok {
		return false, 0, false, false
	}
	strategyType := "RollingUpdate"
	if rawType, exists := strategyMap["type"]; exists {
		strategyType, ok = rawType.(string)
		if !ok {
			return false, 0, false, false
		}
	}

	switch strategyType {
	case "Recreate":
		return replicas > 1, replicas, true, true
	case "RollingUpdate":
		if replicas == 0 {
			return false, 0, true, true
		}
		// Rolling updates can start replacement Pods while old Pods remain in
		// termination. maxSurge therefore does not bound executing processes.
		return true, 0, false, true
	default:
		return false, 0, false, false
	}
}

func nonNegativeInteger(value any) (int, bool) {
	var integer int64
	switch value := value.(type) {
	case int:
		if value < 0 {
			return 0, false
		}
		return value, true
	case int64:
		integer = value
	case uint64:
		converted := int(value)
		if converted < 0 || uint64(converted) != value {
			return 0, false
		}
		return converted, true
	case float64:
		integer = int64(value)
		if value != float64(integer) {
			return 0, false
		}
	default:
		return 0, false
	}
	if integer < 0 {
		return 0, false
	}
	converted := int(integer)
	if int64(converted) != integer {
		return 0, false
	}
	return converted, true
}

func parseResource(resource string) (string, string, error) {
	kind, name, ok := strings.Cut(resource, "/")
	if !ok || kind == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("resource must have the form Kind/name")
	}
	return kind, name, nil
}

func optionString(options map[string]any, key string) (string, error) {
	value, exists := options[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("with.%s must be a string", key)
	}
	return text, nil
}

func manifestPaths(root, source string) ([]string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}
	if !info.IsDir() {
		if !isYAML(source) {
			return nil, fmt.Errorf("source %q is not a YAML file", source)
		}
		return []string{source}, nil
	}

	var paths []string
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isYAML(path) {
			return nil
		}
		resolved, err := withinRoot(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, resolved)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func isYAML(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func decodeFile(path string) ([]manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var manifests []manifest
	decoder := yaml.NewDecoder(file)
	for {
		var value manifest
		err := decoder.Decode(&value)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if value.APIVersion == "" && value.Kind == "" {
			continue
		}
		if value.Kind == "List" {
			manifests = append(manifests, value.Items...)
			continue
		}
		manifests = append(manifests, value)
	}
	return manifests, nil
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
