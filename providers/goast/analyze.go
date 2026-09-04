// Package goast is a deliberately small, language-specific code-fact provider.
// Nothing in Waldo core imports this package.
package goast

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mirage-security/waldo/protocol"
)

const CriticalDeferredWorkDirective = "waldo:correctness-critical-deferred-work"

type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	CgoFiles   []string
}

// Analyze emits a deferred-execution fact for every compiled time.AfterFunc
// call. Criticality is explicit on the enclosing function's doc comment; the
// provider does not assume that every timer is correctness-critical.
func Analyze(ctx context.Context, root string) ([]protocol.CodeFact, error) {
	packages, err := listPackages(ctx, root)
	if err != nil {
		return nil, err
	}
	var facts []protocol.CodeFact
	for _, packageInfo := range packages {
		files := append(append([]string(nil), packageInfo.GoFiles...), packageInfo.CgoFiles...)
		for _, name := range files {
			path := filepath.Join(packageInfo.Dir, name)
			fileFacts, err := analyzeFile(root, packageInfo.ImportPath, path)
			if err != nil {
				return nil, err
			}
			facts = append(facts, fileFacts...)
		}
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].ID < facts[j].ID })
	return facts, nil
}

func listPackages(ctx context.Context, root string) ([]listedPackage, error) {
	command := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("go list: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	decoder := json.NewDecoder(&stdout)
	var packages []listedPackage
	for {
		var packageInfo listedPackage
		if err := decoder.Decode(&packageInfo); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		packages = append(packages, packageInfo)
	}
	return packages, nil
}

func analyzeFile(root, importPath, filename string) ([]protocol.CodeFact, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	timeNames := importedTimeNames(file)
	if len(timeNames) == 0 {
		return nil, nil
	}
	relativePath, err := filepath.Rel(root, filename)
	if err != nil {
		return nil, err
	}
	relativePath = filepath.ToSlash(relativePath)

	var facts []protocol.CodeFact
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		critical := hasDirective(function.Doc, CriticalDeferredWorkDirective)
		symbol := functionSymbol(function)
		occurrence := 0
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isTimeAfterFunc(call.Fun, timeNames) {
				return true
			}
			occurrence++
			position := fileSet.Position(call.Pos())
			factID := strings.Join([]string{
				"go", importPath, symbol, "time.AfterFunc", strconv.Itoa(occurrence),
			}, ":")
			facts = append(facts, protocol.CodeFact{
				ID:     factID,
				Kind:   "deferred-execution",
				Source: protocol.SourceLocation{Path: relativePath, Line: position.Line, Column: position.Column},
				Symbol: importPath + "." + symbol,
				Attributes: map[string]any{
					"correctness.critical": critical,
					"execution.authority":  "process-local",
					"execution.mechanism":  "process-local-timer",
					"language":             "go",
				},
			})
			return true
		})
	}
	return facts, nil
}

func importedTimeNames(file *ast.File) map[string]struct{} {
	names := make(map[string]struct{})
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "time" {
			continue
		}
		name := "time"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "_" && name != "." {
			names[name] = struct{}{}
		}
	}
	return names
}

func isTimeAfterFunc(expression ast.Expr, timeNames map[string]struct{}) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "AfterFunc" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = timeNames[identifier.Name]
	return ok
}

func hasDirective(comments *ast.CommentGroup, directive string) bool {
	if comments == nil {
		return false
	}
	for _, comment := range comments.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		if text == directive {
			return true
		}
	}
	return false
}

func functionSymbol(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	return receiverSymbol(function.Recv.List[0].Type) + "." + function.Name.Name
}

func receiverSymbol(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return receiverSymbol(expression.X)
	case *ast.IndexExpr:
		return receiverSymbol(expression.X)
	case *ast.IndexListExpr:
		return receiverSymbol(expression.X)
	default:
		return "receiver"
	}
}
