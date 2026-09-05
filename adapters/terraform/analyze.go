// Package terraform statically translates Terraform configuration into
// analyzer-neutral deployment facts. It never executes Terraform, loads
// providers, reads state, or contacts a backend.
package terraform

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mirage-security/waldo/protocol"
	"github.com/zclconf/go-cty/cty"
)

type module struct {
	dir     string
	body    *hclsyntax.Body
	context *hcl.EvalContext
}

// Analyze resolves a selected Terraform resource or module using only files
// beneath request.Root. Options currently accepts varFiles, a list of paths
// relative to request.Source.
func Analyze(request protocol.DeploymentRequest) (protocol.DeploymentResult, error) {
	if _, err := withinRoot(request.Root, request.Source); err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("source: %w", err)
	}
	info, err := os.Stat(request.Source)
	if err != nil {
		return protocol.DeploymentResult{}, fmt.Errorf("inspect source: %w", err)
	}
	if !info.IsDir() {
		return protocol.DeploymentResult{}, fmt.Errorf("source must be a Terraform directory")
	}
	varFiles, err := optionStrings(request.Options, "varFiles")
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	rootModule, err := loadModule(request.Root, request.Source, nil, varFiles)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	segments := strings.Split(request.Resource, ".")
	if len(segments) < 2 {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q must be a Terraform address", request.Resource)
	}
	facts, found, err := inspectAddress(request.Root, rootModule, segments)
	if err != nil {
		return protocol.DeploymentResult{}, err
	}
	if !found {
		return protocol.DeploymentResult{}, fmt.Errorf("resource %q not found", request.Resource)
	}
	return protocol.DeploymentResult{Facts: facts}, nil
}

func inspectAddress(root string, current *module, address []string) (map[string]any, bool, error) {
	kind, name := address[0], unindex(address[1])
	block := findBlock(current.body, kind, name)
	if block == nil {
		return nil, false, nil
	}
	if enabled, known := blockEnabled(block, current.context); known && !enabled {
		return map[string]any{}, true, nil
	} else if !known {
		return map[string]any{}, true, nil
	}
	if kind == "module" {
		child, remote, err := loadChild(root, current, block)
		if err != nil {
			return nil, false, err
		}
		if len(address) > 2 {
			if remote != "" {
				return nil, false, fmt.Errorf("resource address enters remote module %q, whose source is unavailable", remote)
			}
			return inspectAddress(root, child, address[2:])
		}
		if remote != "" {
			return factsForRemoteModule(remote, block, current.context), true, nil
		}
		facts, err := scanModule(root, child, map[string]bool{})
		return facts, true, err
	}
	return factsForResource(kind, block, current.context), true, nil
}

func scanModule(root string, current *module, visiting map[string]bool) (map[string]any, error) {
	if visiting[current.dir] {
		return map[string]any{}, nil
	}
	visiting[current.dir] = true
	defer delete(visiting, current.dir)
	facts := make(map[string]any)
	for _, block := range current.body.Blocks {
		if enabled, known := blockEnabled(block, current.context); !known || !enabled {
			continue
		}
		switch block.Type {
		case "resource":
			if len(block.Labels) >= 2 {
				if err := mergeFacts(facts, factsForResource(block.Labels[0], block, current.context)); err != nil {
					return nil, err
				}
			}
		case "module":
			child, remote, err := loadChild(root, current, block)
			if err != nil {
				return nil, err
			}
			if remote != "" {
				if err := mergeFacts(facts, factsForRemoteModule(remote, block, current.context)); err != nil {
					return nil, err
				}
				continue
			}
			childFacts, err := scanModule(root, child, visiting)
			if err != nil {
				return nil, err
			}
			if err := mergeFacts(facts, childFacts); err != nil {
				return nil, err
			}
		}
	}
	return facts, nil
}

func loadModule(root, dir string, supplied map[string]cty.Value, varFiles []string) (*module, error) {
	body, err := parseDirectory(dir)
	if err != nil {
		return nil, err
	}
	variables := make(map[string]cty.Value)
	for _, block := range body.Blocks {
		if block.Type != "variable" || len(block.Labels) != 1 {
			continue
		}
		if attribute := block.Body.Attributes["default"]; attribute != nil {
			if value, ok := evaluateNullable(attribute.Expr, nil); ok {
				variables[block.Labels[0]] = value
			}
		}
	}
	for name, value := range supplied {
		variables[name] = value
	}
	for _, configuredPath := range varFiles {
		path := configuredPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, filepath.FromSlash(path))
		}
		if _, err := withinRoot(root, path); err != nil {
			return nil, fmt.Errorf("varFiles entry %q: %w", configuredPath, err)
		}
		values, err := parseVarFile(path)
		if err != nil {
			return nil, err
		}
		for name, value := range values {
			variables[name] = value
		}
	}
	context := &hcl.EvalContext{Variables: map[string]cty.Value{"var": objectValue(variables)}}
	locals := make(map[string]cty.Value)
	localCount := 0
	for _, block := range body.Blocks {
		if block.Type == "locals" {
			localCount += len(block.Body.Attributes)
		}
	}
	for range localCount {
		progress := false
		context.Variables["local"] = objectValue(locals)
		for _, block := range body.Blocks {
			if block.Type != "locals" {
				continue
			}
			for name, attribute := range block.Body.Attributes {
				if _, exists := locals[name]; exists {
					continue
				}
				if value, ok := evaluate(attribute.Expr, context); ok {
					locals[name] = value
					progress = true
				}
			}
		}
		if !progress {
			break
		}
	}
	context.Variables["local"] = objectValue(locals)
	return &module{dir: filepath.Clean(dir), body: body, context: context}, nil
}

func loadChild(root string, parent *module, block *hclsyntax.Block) (*module, string, error) {
	sourceAttribute := block.Body.Attributes["source"]
	if sourceAttribute == nil {
		return nil, "", fmt.Errorf("module %q has no source", block.Labels[0])
	}
	sourceValue, ok := evaluate(sourceAttribute.Expr, parent.context)
	if !ok || sourceValue.Type() != cty.String {
		return nil, "", fmt.Errorf("module %q source is not statically known", block.Labels[0])
	}
	source := sourceValue.AsString()
	if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
		return nil, source, nil
	}
	childDir := filepath.Join(parent.dir, filepath.FromSlash(source))
	if _, err := withinRoot(root, childDir); err != nil {
		return nil, "", fmt.Errorf("module %q: %w", block.Labels[0], err)
	}
	inputs := make(map[string]cty.Value)
	for name, attribute := range block.Body.Attributes {
		if name == "source" || name == "version" {
			continue
		}
		if value, ok := evaluateNullable(attribute.Expr, parent.context); ok {
			inputs[name] = value
		} else {
			// A supplied-but-unknown value must not accidentally fall back to
			// the child variable's default.
			inputs[name] = cty.DynamicVal
		}
	}
	child, err := loadModule(root, childDir, inputs, nil)
	if err != nil {
		return nil, "", fmt.Errorf("module %q: %w", block.Labels[0], err)
	}
	return child, "", nil
}

func factsForRemoteModule(source string, block *hclsyntax.Block, context *hcl.EvalContext) map[string]any {
	switch {
	case strings.Contains(source, "terraform-aws-modules/ecs/aws"):
		return ecsFacts(block.Body.Attributes, context)
	case strings.Contains(source, "terraform-aws-modules/lambda/aws"):
		return lambdaFacts(block.Body.Attributes, context)
	default:
		return map[string]any{}
	}
}

func factsForResource(resourceType string, block *hclsyntax.Block, context *hcl.EvalContext) map[string]any {
	switch resourceType {
	case "aws_ecs_service":
		return ecsFacts(block.Body.Attributes, context)
	case "aws_lambda_function":
		return lambdaFacts(block.Body.Attributes, context)
	default:
		return map[string]any{}
	}
}

func ecsFacts(attributes hclsyntax.Attributes, context *hcl.EvalContext) map[string]any {
	facts := map[string]any{
		"memory.scope":                    "instance",
		"platform.executionModel":         "orchestrated-container",
		"process.restartable":             true,
		"scheduling.processLocal.durable": false,
	}
	concurrency, concurrencyKnown := knownIntegerDeep(attributes, context, "desired_count")
	if !concurrencyKnown {
		concurrency, concurrencyKnown = knownIntegerDeep(attributes, context, "autoscaling_min_capacity")
	}
	if enabled, ok := knownBoolDeep(attributes, context, "enable_autoscaling"); ok && enabled {
		if maximum := knownIntDeep(attributes, context, "autoscaling_max_capacity"); maximum > concurrency {
			concurrency = maximum
			concurrencyKnown = true
		}
	}
	if concurrencyKnown && concurrency > 0 {
		maximumPercent := knownIntDeep(attributes, context, "deployment_maximum_percent")
		if maximumPercent == 0 {
			// AWS ECS replica services default to allowing 200% of desired
			// count during a rolling deployment.
			maximumPercent = 200
		}
		if maximumPercent > 100 {
			concurrency = concurrency * maximumPercent / 100
		}
		facts["deployment.replicas.concurrent"] = concurrency > 1
		facts["deployment.replicas.maxConcurrent"] = concurrency
	} else if concurrencyKnown && concurrency == 0 {
		facts["deployment.replicas.concurrent"] = false
		facts["deployment.replicas.maxConcurrent"] = 0
	}
	return facts
}

func lambdaFacts(attributes hclsyntax.Attributes, context *hcl.EvalContext) map[string]any {
	facts := map[string]any{
		"memory.scope":                    "instance",
		"platform.executionModel":         "request-scoped-function",
		"process.restartable":             true,
		"scheduling.processLocal.durable": false,
	}
	reserved, hasReserved := knownInteger(attributes, context, "reserved_concurrent_executions")
	if hasReserved && reserved >= 0 {
		facts["deployment.replicas.concurrent"] = reserved > 1
		facts["deployment.replicas.maxConcurrent"] = reserved
	} else {
		facts["deployment.replicas.concurrent"] = true
	}
	return facts
}

func knownIntDeep(attributes hclsyntax.Attributes, context *hcl.EvalContext, name string) int {
	maximum, found := knownIntegerDeep(attributes, context, name)
	if !found || maximum < 0 {
		return 0
	}
	return maximum
}

func knownIntegerDeep(attributes hclsyntax.Attributes, context *hcl.EvalContext, name string) (int, bool) {
	maximum, found := knownInteger(attributes, context, name)
	for _, attribute := range attributes {
		for _, expression := range namedObjectValues(attribute.Expr, context, name) {
			value, ok := evaluate(expression, context)
			if !ok || value.Type() != cty.Number {
				continue
			}
			integer, accuracy := value.AsBigFloat().Int64()
			if accuracy == 0 && (!found || int(integer) > maximum) {
				maximum = int(integer)
				found = true
			}
		}
	}
	return maximum, found
}

func knownInteger(attributes hclsyntax.Attributes, context *hcl.EvalContext, name string) (int, bool) {
	attribute := attributes[name]
	if attribute == nil {
		return 0, false
	}
	value, ok := evaluate(attribute.Expr, context)
	if !ok || value.Type() != cty.Number {
		return 0, false
	}
	integer, accuracy := value.AsBigFloat().Int64()
	if accuracy == 0 {
		return int(integer), true
	}
	return 0, false
}

func knownBool(attributes hclsyntax.Attributes, context *hcl.EvalContext, name string) (bool, bool) {
	attribute := attributes[name]
	if attribute == nil {
		return false, false
	}
	value, ok := evaluate(attribute.Expr, context)
	if !ok || value.Type() != cty.Bool {
		return false, false
	}
	return value.True(), true
}

func knownBoolDeep(attributes hclsyntax.Attributes, context *hcl.EvalContext, name string) (bool, bool) {
	if value, ok := knownBool(attributes, context, name); ok {
		return value, true
	}
	found := false
	for _, attribute := range attributes {
		for _, expression := range namedObjectValues(attribute.Expr, context, name) {
			value, ok := evaluate(expression, context)
			if !ok || value.Type() != cty.Bool {
				continue
			}
			found = true
			if value.True() {
				return true, true
			}
		}
	}
	return false, found
}

func namedObjectValues(expression hcl.Expression, context *hcl.EvalContext, name string) []hcl.Expression {
	object, ok := expression.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil
	}
	var matches []hcl.Expression
	for _, item := range object.Items {
		if key, ok := evaluate(item.KeyExpr, context); ok && key.Type() == cty.String && key.AsString() == name {
			matches = append(matches, item.ValueExpr)
		}
		matches = append(matches, namedObjectValues(item.ValueExpr, context, name)...)
	}
	return matches
}

func evaluate(expression hcl.Expression, context *hcl.EvalContext) (cty.Value, bool) {
	value, ok := evaluateNullable(expression, context)
	if !ok || value.IsNull() {
		return cty.NilVal, false
	}
	return value, true
}

func evaluateNullable(expression hcl.Expression, context *hcl.EvalContext) (cty.Value, bool) {
	value, diagnostics := expression.Value(context)
	if diagnostics.HasErrors() || !value.IsKnown() {
		return cty.NilVal, false
	}
	return value, true
}

func blockEnabled(block *hclsyntax.Block, context *hcl.EvalContext) (bool, bool) {
	if attribute := block.Body.Attributes["count"]; attribute != nil {
		value, ok := evaluate(attribute.Expr, context)
		if !ok || value.Type() != cty.Number {
			return false, false
		}
		count, accuracy := value.AsBigFloat().Int64()
		if accuracy != 0 {
			return false, false
		}
		return count > 0, true
	}
	if attribute := block.Body.Attributes["for_each"]; attribute != nil {
		value, ok := evaluate(attribute.Expr, context)
		if !ok || !value.CanIterateElements() {
			return false, false
		}
		return value.LengthInt() > 0, true
	}
	return true, true
}

func parseDirectory(dir string) (*hclsyntax.Body, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}
	filenames := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		filenames = append(filenames, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(filenames)
	merged := &hclsyntax.Body{Attributes: hclsyntax.Attributes{}}
	parser := hclparse.NewParser()
	for _, filename := range filenames {
		file, diagnostics := parser.ParseHCLFile(filename)
		if diagnostics.HasErrors() {
			return nil, fmt.Errorf("parse %s: %s", filename, diagnostics.Error())
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			return nil, fmt.Errorf("parse %s: unexpected HCL body", filename)
		}
		for name, attribute := range body.Attributes {
			merged.Attributes[name] = attribute
		}
		merged.Blocks = append(merged.Blocks, body.Blocks...)
	}
	if len(filenames) == 0 {
		return nil, fmt.Errorf("source contains no .tf files")
	}
	return merged, nil
}

func parseVarFile(path string) (map[string]cty.Value, error) {
	parser := hclparse.NewParser()
	file, diagnostics := parser.ParseHCLFile(path)
	if diagnostics.HasErrors() {
		return nil, fmt.Errorf("parse var file %s: %s", path, diagnostics.Error())
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("parse var file %s: unexpected HCL body", path)
	}
	values := make(map[string]cty.Value)
	for name, attribute := range body.Attributes {
		value, ok := evaluateNullable(attribute.Expr, nil)
		if !ok {
			return nil, fmt.Errorf("var file %s value %q is not statically known", path, name)
		}
		values[name] = value
	}
	return values, nil
}

func findBlock(body *hclsyntax.Body, kind, name string) *hclsyntax.Block {
	for _, block := range body.Blocks {
		if kind == "module" && block.Type == "module" && len(block.Labels) == 1 && block.Labels[0] == name {
			return block
		}
		if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == kind && block.Labels[1] == name {
			return block
		}
	}
	return nil
}

func optionStrings(options map[string]any, name string) ([]string, error) {
	value, exists := options[name]
	if !exists {
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("option %q must be a list of strings", name)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("option %q must be a list of strings", name)
		}
		result = append(result, text)
	}
	return result, nil
}

func objectValue(values map[string]cty.Value) cty.Value {
	if len(values) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(values)
}

func mergeFacts(destination, source map[string]any) error {
	for name, value := range source {
		if current, exists := destination[name]; exists {
			if name == "deployment.replicas.maxConcurrent" {
				currentNumber, currentOK := current.(int)
				valueNumber, valueOK := value.(int)
				if currentOK && valueOK {
					if valueNumber > currentNumber {
						destination[name] = value
					}
					continue
				}
			}
			if !reflect.DeepEqual(current, value) {
				return fmt.Errorf("selected resource expands to conflicting fact %q (%v and %v)", name, current, value)
			}
			continue
		}
		destination[name] = value
	}
	return nil
}

func unindex(name string) string {
	if index := strings.IndexByte(name, '['); index >= 0 {
		return name[:index]
	}
	return name
}

func withinRoot(root, candidate string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve analysis root: %w", err)
	}
	canonicalCandidate, err := filepath.EvalSymlinks(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalCandidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside analysis root", candidate)
	}
	return relative, nil
}
