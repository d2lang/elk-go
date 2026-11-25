// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// constantDefaultIsItsOwnClone reports whether copying a property map leaves
// the property's default value as it is: it has no Clone method and is not
// one of the slices PropertyMap copies element by element.
// CloneablePropertyWithConstantDefault may only declare such properties, and
// stores the default without asking.
func constantDefaultIsItsOwnClone[T any](property basegraph.Property[T]) bool {
	value := any(property.Default())
	if _, cloneable := value.(interface{ Clone() any }); cloneable {
		return false
	}
	switch value.(type) {
	case []int, []float64:
		return false
	}
	return true
}

// TestConstantDefaultsAreTheirOwnClones checks every property this module
// declares with CloneablePropertyWithConstantDefault, and that the list here
// is all of them.
func TestConstantDefaultsAreTheirOwnClones(t *testing.T) {
	checks := map[string]bool{
		"ExternalPortSize":                constantDefaultIsItsOwnClone(ExternalPortSize),
		"TargetOffset":                    constantDefaultIsItsOwnClone(TargetOffset),
		"Padding":                         constantDefaultIsItsOwnClone(Padding),
		"NodeSizeMinimum":                 constantDefaultIsItsOwnClone(NodeSizeMinimum),
		"NodeSizeConstraints":             constantDefaultIsItsOwnClone(NodeSizeConstraints),
		"ContentAlignment":                constantDefaultIsItsOwnClone(ContentAlignment),
		"NodeLabelsPadding":               constantDefaultIsItsOwnClone(NodeLabelsPadding),
		"SpacingPortsSurrounding":         constantDefaultIsItsOwnClone(SpacingPortsSurrounding),
		"nodespacing.Margins":             constantDefaultIsItsOwnClone(nodespacing.Margins),
		"nodespacing.NodeLabelsPlacement": constantDefaultIsItsOwnClone(nodespacing.NodeLabelsPlacement),
		"nodespacing.PortLabelsPlacement": constantDefaultIsItsOwnClone(nodespacing.PortLabelsPlacement),
		"nodespacing.NodeSizeOptions":     constantDefaultIsItsOwnClone(nodespacing.NodeSizeOptions),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s: a copy of its default is not the default itself; declare it with CloneablePropertyWithDefault", name)
		}
	}

	if runtime.Compiler == "tinygo" {
		// What follows reads the module's source, which a WebAssembly test
		// binary cannot.
		return
	}
	declared := 0
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "testdata" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			path == filepath.Join(root, "internal", "graph", "properties.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		declared += bytes.Count(source, []byte("CloneablePropertyWithConstantDefault("))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if declared != len(checks) {
		t.Fatalf("the module declares %d properties with CloneablePropertyWithConstantDefault, this test checks %d", declared, len(checks))
	}
}
