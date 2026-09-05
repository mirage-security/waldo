// Package policies exposes Waldo's built-in architectural policies as data.
//
// Keeping the catalog here preserves the policy boundary: the evaluator remains
// generic, while installed Waldo binaries carry the stable policy set with them.
package policies

import (
	"embed"
	"fmt"
)

//go:embed *.yaml
var catalog embed.FS

// Documents returns the built-in policy documents in deterministic filename
// order. Callers must parse and validate the documents before use.
func Documents() ([][]byte, error) {
	entries, err := catalog.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read built-in policy catalog: %w", err)
	}
	documents := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		document, err := catalog.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read built-in policy %q: %w", entry.Name(), err)
		}
		documents = append(documents, document)
	}
	return documents, nil
}
