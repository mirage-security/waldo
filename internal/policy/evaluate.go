package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/mirage-security/waldo/internal/config"
	"github.com/mirage-security/waldo/internal/model"
)

func Evaluate(configuration config.Config, facts []model.CodeFact) ([]model.Finding, error) {
	dispositions := make(map[string]config.FindingDisposition, len(configuration.Dispositions))
	for _, disposition := range configuration.Dispositions {
		dispositions[disposition.Finding] = disposition
	}

	var findings []model.Finding
	seenFindingIDs := make(map[string]struct{})
	for _, fact := range facts {
		units := matchingUnits(configuration.Deployment.Units, fact.Source.Path)
		for _, unitName := range units {
			unit := configuration.Deployment.Units[unitName]
			for _, rule := range configuration.Policies {
				matchedDeployment, matches, err := matchesPolicy(rule, unit, fact)
				if err != nil {
					return nil, fmt.Errorf("policy %q: %w", rule.ID, err)
				}
				if !matches {
					continue
				}
				finding := model.Finding{
					ID:                FindingID(rule.ID, unitName, fact.Provider, fact.ID),
					PolicyID:          rule.ID,
					PolicyTitle:       rule.Title,
					Severity:          rule.Severity,
					Disposition:       model.DispositionUnresolved,
					DeploymentUnit:    unitName,
					MatchedDeployment: matchedDeployment,
					CodeFact:          fact,
					Message:           rule.Message,
				}
				if disposition, ok := dispositions[finding.ID]; ok {
					finding.Disposition = disposition.Disposition
					finding.DispositionReason = disposition.Reason
				}
				if _, exists := seenFindingIDs[finding.ID]; exists {
					return nil, fmt.Errorf("duplicate finding identity %q; provider fact IDs must be unique", finding.ID)
				}
				seenFindingIDs[finding.ID] = struct{}{}
				findings = append(findings, finding)
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	return findings, nil
}

func FindingID(policyID, deploymentUnit, provider, factID string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"waldo-finding-v1", policyID, deploymentUnit, provider, factID,
	}, "\x00")))
	return "waldo:v1:" + hex.EncodeToString(digest[:])
}

func matchingUnits(units map[string]config.DeploymentUnit, sourcePath string) []string {
	path := filepath.ToSlash(filepath.Clean(sourcePath))
	var matches []string
	for name, unit := range units {
		for _, configuredRoot := range unit.CodeRoots {
			root := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(configuredRoot)), "/")
			if root == "." || path == root || strings.HasPrefix(path, root+"/") {
				matches = append(matches, name)
				break
			}
		}
	}
	sort.Strings(matches)
	return matches
}

func matchesPolicy(rule config.Policy, unit config.DeploymentUnit, fact model.CodeFact) (map[string]any, bool, error) {
	if fact.Kind != rule.When.Code.Kind {
		return nil, false, nil
	}
	for key, expected := range rule.When.Code.Attributes {
		actual, exists := fact.Attributes[key]
		if !exists {
			return nil, false, nil
		}
		matches, err := matchValue(actual, expected)
		if err != nil {
			return nil, false, fmt.Errorf("code attribute %q: %w", key, err)
		}
		if !matches {
			return nil, false, nil
		}
	}

	matched := make(map[string]any, len(rule.When.Deployment))
	for key, expected := range rule.When.Deployment {
		actual, exists := unit.Facts[key]
		if !exists {
			return nil, false, nil
		}
		matches, err := matchValue(actual, expected)
		if err != nil {
			return nil, false, fmt.Errorf("deployment fact %q: %w", key, err)
		}
		if !matches {
			return nil, false, nil
		}
		matched[key] = actual
	}
	return matched, true, nil
}

func matchValue(actual, expected any) (bool, error) {
	operators, isOperators := expected.(map[string]any)
	if !isOperators {
		return valuesEqual(actual, expected), nil
	}
	for operator, operand := range operators {
		switch operator {
		case "equals":
			if !valuesEqual(actual, operand) {
				return false, nil
			}
		case "notEquals":
			if valuesEqual(actual, operand) {
				return false, nil
			}
		case "greaterThan", "greaterThanOrEqual", "lessThan", "lessThanOrEqual":
			actualNumber, actualOK := number(actual)
			operandNumber, operandOK := number(operand)
			if !actualOK || !operandOK {
				return false, fmt.Errorf("operator %s requires numbers", operator)
			}
			matches := map[string]bool{
				"greaterThan":        actualNumber > operandNumber,
				"greaterThanOrEqual": actualNumber >= operandNumber,
				"lessThan":           actualNumber < operandNumber,
				"lessThanOrEqual":    actualNumber <= operandNumber,
			}[operator]
			if !matches {
				return false, nil
			}
		case "oneOf":
			values, ok := operand.([]any)
			if !ok {
				return false, fmt.Errorf("operator oneOf requires a list")
			}
			found := false
			for _, value := range values {
				found = found || valuesEqual(actual, value)
			}
			if !found {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unknown operator %q", operator)
		}
	}
	return true, nil
}

func valuesEqual(left, right any) bool {
	leftNumber, leftOK := number(left)
	rightNumber, rightOK := number(right)
	if leftOK && rightOK {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func number(value any) (float64, bool) {
	switch value := value.(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint64:
		return float64(value), true
	case float64:
		return value, true
	case float32:
		return float64(value), true
	default:
		return 0, false
	}
}
