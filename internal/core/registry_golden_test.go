// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core_test

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/basic"
	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/mrtree"
	"github.com/d2lang/elk-go/internal/radial"
	"github.com/d2lang/elk-go/internal/rectpacking"
	"github.com/d2lang/elk-go/internal/spore"
	"github.com/d2lang/elk-go/internal/stress"
)

//go:embed testdata/default_registry_options.golden testdata/default_registry_keys.golden testdata/package_registry_options.golden
var registryGolden embed.FS

// updateGoldenEnv names the environment variable that makes the test rewrite
// its golden files instead of comparing with them (a variable and not a flag,
// so that it also reaches a test run by TinyGo).
const updateGoldenEnv = "ELK_GO_UPDATE_GOLDEN"

// defaultRegistrySnapshot is taken while this package initialises: after
// every package has declared its properties and before any test runs, as in a
// program that calls Layout for the first time.
var defaultRegistrySnapshot = snapshotDefaultRegistry()

type registrySnapshot struct {
	options  string
	keys     string
	packages string
}

func snapshotDefaultRegistry() registrySnapshot {
	// The registries, as elk.defaultRegistries builds them.
	registry := core.NewOptionRegistry()
	builtins.RegisterOptions(registry)

	var options strings.Builder
	for index, option := range registry.Options() {
		fmt.Fprintf(&options, "%d %s\n", index, describeOption(registry, option))
	}

	// Every key interned so far, in key order: a new ID gets the next key.
	next := graph.InternPropertyKey("test.core.registry-golden.sentinel")
	var keys strings.Builder
	for key := graph.PropertyKey(1); key < next; key++ {
		var properties graph.PropertyMap
		properties.SetRawKey(key, nil)
		fmt.Fprintf(&keys, "%d %s\n", key, properties.Entries()[0].ID)
	}
	return registrySnapshot{options: options.String(), keys: keys.String(), packages: snapshotPackageRegistries()}
}

// snapshotPackageRegistries describes what each algorithm package registers
// by itself, on top of the core options of a new registry.
func snapshotPackageRegistries() string {
	var packages strings.Builder
	coreOptions := len(core.NewOptionRegistry().Options())
	for _, register := range []struct {
		name     string
		register func(*core.OptionRegistry)
	}{
		{"core", func(*core.OptionRegistry) {}},
		{"basic", basic.RegisterOptions},
		{"layered", layered.RegisterOptions},
		{"force", force.RegisterOptions},
		{"stress", stress.RegisterOptions},
		{"mrtree", mrtree.RegisterOptions},
		{"radial", radial.RegisterOptions},
		{"spore", spore.RegisterOptions},
		{"rectpacking", rectpacking.RegisterOptions},
	} {
		registry := core.NewOptionRegistry()
		register.register(registry)
		// Registering twice changes nothing: options that are registered
		// are skipped.
		if register.name != "core" {
			register.register(registry)
		}
		options := registry.Options()
		from := coreOptions
		if register.name == "core" {
			from = 0
		}
		fmt.Fprintf(&packages, "# %s: %d options\n", register.name, len(options))
		for index := from; index < len(options); index++ {
			fmt.Fprintf(&packages, "%s %d %s\n", register.name, index, describeOption(registry, options[index]))
		}
	}
	return packages.String()
}

var optionKindNames = map[core.OptionKind]string{
	core.StringOption: "string", core.BooleanOption: "boolean", core.IntegerOption: "integer",
	core.DoubleOption: "double", core.EnumOption: "enum", core.EnumSetOption: "enumset",
	core.ObjectOption: "object", core.UndefinedOption: "undefined",
}

func describeTargets(targets core.Targets) string {
	var names []string
	for _, target := range []struct {
		bit  core.Targets
		name string
	}{{core.Parents, "parents"}, {core.Nodes, "nodes"}, {core.Edges, "edges"}, {core.Ports, "ports"}, {core.Labels, "labels"}} {
		if targets&target.bit != 0 {
			names = append(names, target.name)
			targets &^= target.bit
		}
	}
	if targets != 0 {
		names = append(names, fmt.Sprintf("unknown(%d)", targets))
	}
	return strings.Join(names, "+")
}

// describeOption is every field of a registered option, and what it parses a
// set of sample texts into (as a digest: the texts tell the parsers of the
// options apart).
func describeOption(registry *core.OptionRegistry, option *core.OptionData) string {
	var flags []string
	if option.MetadataOnly {
		flags = append(flags, "metadataOnly")
	}
	if option.PureParse() {
		flags = append(flags, "pureParse")
	}
	if option.Parse != nil {
		flags = append(flags, "parse")
	}
	if option.Format != nil {
		flags = append(flags, "format")
	}
	var properties graph.PropertyMap
	properties.SetRawKey(option.RegisteredKey(), nil)
	keyName := properties.Entries()[0].ID
	if keyName != option.ID {
		flags = append(flags, "key="+keyName)
	}
	if registry.ByID(option.ID) != option {
		flags = append(flags, "notByID")
	}
	for _, legacy := range option.LegacyIDs {
		if registry.ByID(legacy) != option {
			flags = append(flags, "notByLegacyID")
		}
	}

	samples := []string{
		"", "null", "true", "TRUE", "false", "yes", "0", "1", "2", "7", "-3", "99", " 4 ", "1.5", "1.5f", "2D",
		"1e3", "abc", "NONE", "[]", "[1, 2,3]", "1 2", "(1,2)", "1,2", "(1,2; 3,4)", "{1,2}",
		"[top=1,left=2,bottom=3,right=4]", "[left=5]", "top=1", "{}",
		"org.eclipse.elk.spacing.nodeNode:5", "spacing.nodeNode:5;,;edgeNode:3", "nodeNode:x", "individual:1",
	}
	samples = append(samples, option.EnumValues...)
	if len(option.EnumValues) > 0 {
		samples = append(samples,
			strings.ToLower(option.EnumValues[0]),
			option.EnumValues[0]+", "+option.EnumValues[len(option.EnumValues)-1],
			"["+strings.Join(option.EnumValues, ",")+"]",
			" "+option.EnumValues[0],
		)
	}
	digest := sha256.New()
	for _, sample := range samples {
		value, valid := option.ParseValue(sample)
		formatted := option.FormatValue(value)
		if _, spacings := value.(*core.IndividualSpacings); spacings && option.Format == nil {
			// Without a Format function this is fmt.Sprint of a property map,
			// which prints the numbers of its keys. describeValue has what
			// the map holds.
			formatted = ""
		}
		fmt.Fprintf(digest, "%q -> %v %s | %q\n", sample, valid, describeValue(value), formatted)
	}

	return fmt.Sprintf("%s | group=%s | kind=%s | targets=%s | flags=%s | legacy=%s | enum=%s | parses=%x",
		option.ID, option.Group, optionKindNames[option.Kind], describeTargets(option.Targets),
		strings.Join(flags, ","), strings.Join(option.LegacyIDs, ","), strings.Join(option.EnumValues, ","),
		digest.Sum(nil)[:8])
}

func describeValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "nil"
	case string, bool, int, float64, []string, []int:
		return fmt.Sprintf("%T %#v", value, value)
	case geometry.Point, geometry.Spacing, geometry.Chain:
		return fmt.Sprintf("%T %v", value, value)
	case *core.IndividualSpacings:
		var entries []string
		for _, entry := range typed.Properties.Entries() {
			entries = append(entries, entry.ID+"="+describeValue(entry.Value))
		}
		return "*IndividualSpacings{" + strings.Join(entries, "; ") + "}"
	default:
		return fmt.Sprintf("unexpected %T %v", value, value)
	}
}

// TestDefaultRegistryGolden pins the default option registry (every option,
// in registration order, with all of its metadata and what it parses), what
// each algorithm package registers by itself, and the table of property keys
// behind them (every ID interned by a property declaration or by registering
// the options, in key order).
//
// The files change whenever an option or a property is added, renamed or
// reordered: run the test with ELK_GO_UPDATE_GOLDEN=1 and review the
// difference. They must not change otherwise.
func TestDefaultRegistryGolden(t *testing.T) {
	check := func(name, got string) {
		t.Helper()
		path := "testdata/" + name
		if os.Getenv(updateGoldenEnv) != "" {
			if err := os.WriteFile(filepath.FromSlash(path), []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		want, err := registryGolden.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got == string(want) {
			return
		}
		gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for index := 0; index < len(gotLines) || index < len(wantLines); index++ {
			var gotLine, wantLine string
			if index < len(gotLines) {
				gotLine = gotLines[index]
			}
			if index < len(wantLines) {
				wantLine = wantLines[index]
			}
			if gotLine != wantLine {
				t.Fatalf("%s differs at line %d:\n got: %s\nwant: %s\n(%d lines, want %d; set %s=1 after a deliberate change)",
					path, index+1, gotLine, wantLine, len(gotLines), len(wantLines), updateGoldenEnv)
			}
		}
	}
	check("default_registry_options.golden", defaultRegistrySnapshot.options)
	check("package_registry_options.golden", defaultRegistrySnapshot.packages)
	if runtime.Compiler == "tinygo" {
		// Keys are assigned in the order in which packages initialise, and
		// TinyGo orders them differently from the Go toolchain (both orders
		// are fixed). The golden file holds the Go toolchain's.
		t.Logf("property keys in TinyGo's order:\n%s", defaultRegistrySnapshot.keys)
		return
	}
	check("default_registry_keys.golden", defaultRegistrySnapshot.keys)
}
