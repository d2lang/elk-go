// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// releaseSemanticCase is a deliberately small executable specification of an
// observable behavior added or corrected between ELK 0.8.1 and 0.12.0. The
// upstream field names the exact ELK change that motivated the case. Variants
// differ only in layout options, so stripping layoutOptions from the oracle
// results proves that the option changes actual layout rather than merely
// round-tripping input metadata.
type releaseSemanticCase struct {
	name       string
	claim      string
	upstream   string
	observable bool
	oracleNoOp bool
	variants   []releaseSemanticVariant
}

// TestOracleCMEnforcedGroupOrdersJSONError records an official elkjs 0.12
// surface limitation: the generated JSON importer cannot instantiate the
// List<Integer> property type. The option is advertised by metadata, but an
// ordinary JSON array is rejected before layout begins.
func TestOracleCMEnforcedGroupOrdersJSONError(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE_VALIDATE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE_VALIDATE=1 to validate 0.12 semantic probes")
	}
	input := cmEnforcedGroupOrdersErrorInput(t)
	runner := DefaultRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := runner.Layout(ctx, input)
	if err == nil {
		t.Fatal("official elkjs 0.12 accepted cmEnforcedGroupOrders JSON array; expected generated-importer error")
	}
	const want = "run elkjs 0.12.0 oracle: Error: java.lang.Error: Severe implementation error in the Json to ElkGraph importer."
	if got := oracleErrorHeadline(err); got != want {
		t.Fatalf("official error headline = %q, want %q", got, want)
	}
}

// TestOracleTopdownSizeApproximatorJSONError records the same generated-
// importer limitation for ITopdownSizeApproximator. The Java recursive engine
// implements the strategies, but official elkjs 0.12 cannot construct this
// interface-valued option from JSON.
func TestOracleTopdownSizeApproximatorJSONError(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE_VALIDATE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE_VALIDATE=1 to validate 0.12 semantic probes")
	}
	input := topdownSizeApproximatorErrorInput(t)
	runner := DefaultRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := runner.Layout(ctx, input)
	if err == nil {
		t.Fatal("official elkjs 0.12 accepted topdown.sizeApproximator; expected ElkReflect importer error")
	}
	const want = "run elkjs 0.12.0 oracle: Error: java.lang.IllegalStateException: Couldn't create new instance of property 'org.eclipse.elk.topdown.sizeApproximator'. Make sure its type is registered with the ElkReflect utility class."
	if got := oracleErrorHeadline(err); got != want {
		t.Fatalf("official error headline = %q, want %q", got, want)
	}
}

// TestReleaseImporterErrorParity pins the stable semantic part of two official
// generated-importer failures. JavaScript stack frames and asset paths are not
// part of the Go API, but LayoutJSON returns the same message text.
func TestReleaseImporterErrorParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE=1 to run 0.12 importer-error parity")
	}
	tests := []struct {
		name  string
		input func(*testing.T) []byte
		want  string
	}{
		{
			name:  "cmEnforcedGroupOrders",
			input: cmEnforcedGroupOrdersErrorInput,
			want:  "import ELK graph: Severe implementation error in the Json to ElkGraph importer.",
		},
		{
			name:  "topdownSizeApproximator",
			input: topdownSizeApproximatorErrorInput,
			want:  "import ELK graph: Couldn't create new instance of property 'org.eclipse.elk.topdown.sizeApproximator'. Make sure its type is registered with the ElkReflect utility class.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := elk.LayoutJSON(test.input(t)); err == nil {
				t.Fatalf("LayoutJSON succeeded, want %q", test.want)
			} else if err.Error() != test.want {
				t.Fatalf("LayoutJSON error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestOracleTopdownConfigurationErrors(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE_VALIDATE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE_VALIDATE=1 to validate 0.12 semantic probes")
	}
	runner := DefaultRunner()
	tests := topdownConfigurationErrorInputs(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := runner.Layout(ctx, test.input)
			if err == nil {
				t.Fatalf("official elkjs 0.12 accepted invalid topdown configuration; want %q", test.message)
			}
			want := "run elkjs 0.12.0 oracle: Error: org.eclipse.elk.core.UnsupportedConfigurationException: " + test.message
			if got := oracleErrorHeadline(err); got != want {
				t.Fatalf("official error headline = %q, want %q", got, want)
			}
		})
	}
}

func TestReleaseTopdownConfigurationErrorParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE=1 to run 0.12 topdown-error parity")
	}
	for _, test := range topdownConfigurationErrorInputs(t) {
		t.Run(test.name, func(t *testing.T) {
			if _, err := elk.LayoutJSON(test.input); err == nil {
				t.Fatalf("LayoutJSON succeeded, want %q", test.message)
			} else if want := "layout ELK graph: " + test.message; err.Error() != want {
				t.Fatalf("LayoutJSON error = %q, want %q", err, want)
			}
		})
	}
}

type topdownConfigurationErrorCase struct {
	name, message string
	input         []byte
}

func topdownConfigurationErrorInputs(t *testing.T) []topdownConfigurationErrorCase {
	t.Helper()
	missingType := semanticGraph("fixed", semanticOptions("elk.topdownLayout", "true"),
		[]map[string]any{semanticNode("a", 20, 20, nil)}, nil)
	includeChildren := semanticGraph("layered", semanticOptions(
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "ROOT_NODE",
		"elk.hierarchyHandling", "INCLUDE_CHILDREN",
	), []map[string]any{semanticNode("a", 20, 20, nil)}, nil)
	negativeArea := semanticGraph("fixed", semanticOptions(
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.padding", "[top=6,left=6,bottom=6,right=6]",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
	), []map[string]any{semanticNode("a", 20, 20, nil)}, nil)
	negativeArea["width"], negativeArea["height"] = 10, 10

	inputs := []struct {
		name, message string
		graph         map[string]any
	}{
		{"missing-node-type", "root has not been assigned a top-down node type.", missingType},
		{"include-children", "Topdown layout cannot be used together with hierarchy handling.", includeChildren},
		{"negative-child-area", "The size defined by the parent parallel node is too small for the space provided by the paddings of the child hierarchical node. root", negativeArea},
	}
	result := make([]topdownConfigurationErrorCase, 0, len(inputs))
	for _, input := range inputs {
		encoded, err := json.Marshal(input.graph)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, topdownConfigurationErrorCase{name: input.name, message: input.message, input: encoded})
	}
	return result
}

func cmEnforcedGroupOrdersErrorInput(t *testing.T) []byte {
	t.Helper()
	base := semanticGraph("layered", semanticOptions(
		"elk.layered.considerModelOrder.strategy", "NODES_AND_EDGES",
		"elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy", "ENFORCED",
	), []map[string]any{
		semanticNode("a", 30, 30, nil), semanticNode("b", 30, 30, nil),
	}, []map[string]any{semanticEdge("ab", "a", "b", nil)})
	base["layoutOptions"].(map[string]any)["elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders"] = []int{1, 2}
	input, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func topdownSizeApproximatorErrorInput(t *testing.T) []byte {
	t.Helper()
	compound := semanticNode("compound", 80, 60, semanticOptions(
		"elk.algorithm", "mrtree",
		"elk.nodeSize.fixedGraphSize", "true",
		"elk.topdownLayout", "true",
		"elk.topdown.nodeType", "HIERARCHICAL_NODE",
		"elk.topdown.sizeApproximator", "COUNT_CHILDREN",
	))
	compound["children"] = []map[string]any{semanticNode("inner", 20, 20, semanticOptions(
		"elk.topdownLayout", "true", "elk.topdown.nodeType", "HIERARCHICAL_NODE"))}
	base := semanticGraph("layered", semanticOptions(
		"elk.topdownLayout", "true", "elk.topdown.nodeType", "ROOT_NODE",
	), []map[string]any{compound}, nil)
	input, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func oracleErrorHeadline(err error) string {
	headline, _, _ := strings.Cut(err.Error(), "\n")
	return headline
}

type releaseSemanticVariant struct {
	name  string
	input []byte
}

type semanticEdit func(map[string]any)

func TestReleaseSemanticCorpusDefinition(t *testing.T) {
	cases := releaseSemanticCases()
	seenCases := make(map[string]struct{}, len(cases))
	for _, testCase := range cases {
		if testCase.name == "" || testCase.claim == "" || testCase.upstream == "" {
			t.Fatalf("release semantic case has incomplete provenance: %#v", testCase)
		}
		if _, exists := seenCases[testCase.name]; exists {
			t.Fatalf("duplicate release semantic case %q", testCase.name)
		}
		seenCases[testCase.name] = struct{}{}
		if len(testCase.variants) == 0 {
			t.Fatalf("release semantic case %q has no variants", testCase.name)
		}
		if testCase.observable && len(testCase.variants) < 2 {
			t.Fatalf("observable release semantic case %q needs a baseline and variant", testCase.name)
		}
		if testCase.oracleNoOp && len(testCase.variants) < 2 {
			t.Fatalf("oracle-no-op release semantic case %q needs a baseline and variant", testCase.name)
		}
		if testCase.observable && testCase.oracleNoOp {
			t.Fatalf("release semantic case %q cannot be both observable and an oracle no-op", testCase.name)
		}
		seenVariants := make(map[string]struct{}, len(testCase.variants))
		for _, variant := range testCase.variants {
			if variant.name == "" || !json.Valid(variant.input) {
				t.Fatalf("release semantic case %q has invalid variant %q", testCase.name, variant.name)
			}
			if _, exists := seenVariants[variant.name]; exists {
				t.Fatalf("release semantic case %q repeats variant %q", testCase.name, variant.name)
			}
			seenVariants[variant.name] = struct{}{}
		}
	}
}

// TestOracleReleaseSemanticCasesNoOp pins options that are present in 0.12
// metadata but have no observable effect through elkjs LayoutJSON. Most are
// service-side annotations whose consumers are not part of the JSON layout
// entry point. Keeping these separate prevents a metadata-only option from
// being mistaken for an incomplete native implementation.
func TestOracleReleaseSemanticCasesNoOp(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE_VALIDATE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE_VALIDATE=1 to validate 0.12 semantic probes")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range releaseSemanticCases() {
		testCase := testCase
		if !testCase.oracleNoOp {
			continue
		}
		t.Run(testCase.name, func(t *testing.T) {
			outputs := make([][]byte, len(testCase.variants))
			for index, variant := range testCase.variants {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				output, err := runner.Layout(ctx, variant.input)
				cancel()
				if err != nil {
					t.Fatalf("%s: %v\ninput: %s", variant.name, err, variant.input)
				}
				outputs[index] = semanticLayoutProjection(t, output)
			}
			for index := 1; index < len(outputs); index++ {
				if !bytes.Equal(outputs[0], outputs[index]) {
					t.Errorf("variant %q changes the projected official 0.12 layout from baseline %q; %s is observable",
						testCase.variants[index].name, testCase.variants[0].name, testCase.claim)
					if os.Getenv("ELK_GO_ORACLE_RELEASE_DEBUG") == "1" {
						t.Logf("baseline input: %s\nprojected layout: %s", testCase.variants[0].input, outputs[0])
						t.Logf("variant input: %s\nprojected layout: %s", testCase.variants[index].input, outputs[index])
					}
				}
			}
		})
	}
}

// TestOracleReleaseSemanticCasesObservable validates the test corpus itself.
// It catches misspelled, unsupported, or ineffective options before a native
// mismatch is attributed to the Go port.
func TestOracleReleaseSemanticCasesObservable(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE_VALIDATE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE_VALIDATE=1 to validate 0.12 semantic probes")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range releaseSemanticCases() {
		testCase := testCase
		if !testCase.observable {
			continue
		}
		t.Run(testCase.name, func(t *testing.T) {
			outputs := make([][]byte, len(testCase.variants))
			for index, variant := range testCase.variants {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				output, err := runner.Layout(ctx, variant.input)
				cancel()
				if err != nil {
					t.Fatalf("%s: %v\ninput: %s", variant.name, err, variant.input)
				}
				outputs[index] = semanticLayoutProjection(t, output)
			}
			for index := 1; index < len(outputs); index++ {
				if bytes.Equal(outputs[0], outputs[index]) {
					t.Errorf("variant %q has the same projected official 0.12 layout as baseline %q; probe does not establish %s",
						testCase.variants[index].name, testCase.variants[0].name, testCase.claim)
					if os.Getenv("ELK_GO_ORACLE_RELEASE_DEBUG") == "1" {
						t.Logf("baseline input: %s\nprojected layout: %s", testCase.variants[0].input, outputs[0])
						t.Logf("variant input: %s\nprojected layout: %s", testCase.variants[index].input, outputs[index])
					}
				}
			}
		})
	}
}

// TestCandidateReleaseSemanticParity differentially pins each release-line
// behavior to official elkjs 0.12.0. Keep each variant as a sibling subtest so
// one incomplete algorithm feature does not hide the rest of the inventory.
func TestCandidateReleaseSemanticParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_RELEASE") != "1" {
		t.Skip("set ELK_GO_ORACLE_RELEASE=1 to run 0.12 semantic differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range releaseSemanticCases() {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			for _, variant := range testCase.variants {
				variant := variant
				t.Run(variant.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					if err := Differential(ctx, runner, variant.input, elk.LayoutJSON, ExactTolerance); err != nil {
						t.Fatalf("%v\nclaim: %s\nupstream: %s\ninput: %s", err, testCase.claim, testCase.upstream, variant.input)
					}
				})
			}
		})
	}
}

func semanticLayoutProjection(t *testing.T, raw []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode official layout for semantic projection: %v", err)
	}
	stripSemanticInputMetadata(value)
	projected, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode official layout semantic projection: %v", err)
	}
	return projected
}

func stripSemanticInputMetadata(value any) {
	switch value := value.(type) {
	case map[string]any:
		delete(value, "layoutOptions")
		delete(value, "$H")
		for _, child := range value {
			stripSemanticInputMetadata(child)
		}
	case []any:
		for _, child := range value {
			stripSemanticInputMetadata(child)
		}
	}
}

func releaseSemanticCases() []releaseSemanticCase {
	var cases []releaseSemanticCase
	cases = append(cases, boxSemanticCases()...)
	cases = append(cases, forceSemanticCases()...)
	cases = append(cases, mrTreeSemanticCases()...)
	cases = append(cases, radialSemanticCases()...)
	cases = append(cases, rectPackingSemanticCases()...)
	cases = append(cases, layeredSemanticCases()...)
	cases = append(cases, extendedReleaseSemanticCases()...)
	return cases
}

func semanticGraph(algorithm string, options map[string]any, children, edges []map[string]any) map[string]any {
	rootOptions := map[string]any{"elk.algorithm": algorithm}
	for key, value := range options {
		rootOptions[key] = value
	}
	graph := map[string]any{
		"id":            "root",
		"layoutOptions": rootOptions,
		"children":      children,
	}
	if len(edges) > 0 {
		graph["edges"] = edges
	}
	return graph
}

func semanticNode(id string, width, height float64, options map[string]any) map[string]any {
	node := map[string]any{"id": id, "width": width, "height": height}
	if len(options) > 0 {
		node["layoutOptions"] = options
	}
	return node
}

func semanticEdge(id, source, target string, options map[string]any) map[string]any {
	edge := map[string]any{
		"id":      id,
		"sources": []string{source},
		"targets": []string{target},
	}
	if len(options) > 0 {
		edge["layoutOptions"] = options
	}
	return edge
}

func semanticOptions(pairs ...string) map[string]any {
	if len(pairs)%2 != 0 {
		panic("semanticOptions needs key/value pairs")
	}
	options := make(map[string]any, len(pairs)/2)
	for index := 0; index < len(pairs); index += 2 {
		options[pairs[index]] = pairs[index+1]
	}
	return options
}

func semanticVariant(name string, base map[string]any, edits ...semanticEdit) releaseSemanticVariant {
	encoded, err := json.Marshal(base)
	if err != nil {
		panic(err)
	}
	var graph map[string]any
	if err := json.Unmarshal(encoded, &graph); err != nil {
		panic(err)
	}
	for _, edit := range edits {
		edit(graph)
	}
	encoded, err = json.Marshal(graph)
	if err != nil {
		panic(err)
	}
	return releaseSemanticVariant{name: name, input: encoded}
}

func semanticRootOption(key, value string) semanticEdit {
	return func(graph map[string]any) {
		options := graph["layoutOptions"].(map[string]any)
		options[key] = value
	}
}

func semanticRootField(key string, value any) semanticEdit {
	return func(graph map[string]any) { graph[key] = value }
}

func semanticElementOption(id, key, value string) semanticEdit {
	return func(graph map[string]any) {
		element := semanticFindElement(graph, id)
		if element == nil {
			panic(fmt.Sprintf("semantic graph has no element %q", id))
		}
		options, _ := element["layoutOptions"].(map[string]any)
		if options == nil {
			options = make(map[string]any)
			element["layoutOptions"] = options
		}
		options[key] = value
	}
}

func semanticElementField(id, key string, value any) semanticEdit {
	return func(graph map[string]any) {
		element := semanticFindElement(graph, id)
		if element == nil {
			panic(fmt.Sprintf("semantic graph has no element %q", id))
		}
		element[key] = value
	}
}

func semanticFindElement(value any, id string) map[string]any {
	switch value := value.(type) {
	case map[string]any:
		if value["id"] == id {
			return value
		}
		for _, child := range value {
			if found := semanticFindElement(child, id); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range value {
			if found := semanticFindElement(child, id); found != nil {
				return found
			}
		}
	}
	return nil
}

func boxSemanticCases() []releaseSemanticCase {
	base := semanticGraph("box", semanticOptions(
		"elk.box.packingMode", "SIMPLE",
		"elk.spacing.nodeNode", "9",
	), []map[string]any{
		semanticNode("a", 80, 12, nil),
		semanticNode("b", 18, 64, nil),
		semanticNode("c", 37, 31, nil),
		semanticNode("d", 22, 48, nil),
	}, nil)

	return []releaseSemanticCase{
		{
			name:       "box-padding",
			claim:      "Box uses asymmetric graph padding when placing and bounding packed boxes",
			upstream:   "ELK 0.12 Core.melk Box supports org.eclipse.elk.padding",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("tight", base, semanticRootOption("elk.padding", "[top=1,left=2,bottom=3,right=4]")),
				semanticVariant("wide-asymmetric", base, semanticRootOption("elk.padding", "[top=13,left=17,bottom=19,right=23]")),
			},
		},
		{
			name:       "box-priority",
			claim:      "Box places higher-priority nodes before lower-priority nodes",
			upstream:   "ELK 0.12 Core.melk Box priority ordering contract",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("a-first", base,
					semanticElementOption("a", "elk.priority", "4"),
					semanticElementOption("b", "elk.priority", "1")),
				semanticVariant("b-first", base,
					semanticElementOption("a", "elk.priority", "1"),
					semanticElementOption("b", "elk.priority", "4")),
			},
		},
	}
}

func forceSemanticCases() []releaseSemanticCase {
	base := semanticGraph("force", semanticOptions(
		"elk.randomSeed", "7",
		"elk.force.iterations", "80",
		"elk.force.model", "FRUCHTERMAN_REINGOLD",
		"elk.spacing.nodeNode", "35",
	), []map[string]any{
		semanticNode("a", 30, 20, nil),
		semanticNode("b", 35, 25, nil),
		semanticNode("c", 25, 30, nil),
		semanticNode("d", 40, 18, nil),
	}, []map[string]any{
		semanticEdge("e1", "a", "b", nil),
		semanticEdge("e2", "b", "c", nil),
		semanticEdge("e3", "c", "d", nil),
		semanticEdge("e4", "d", "a", nil),
	})

	return []releaseSemanticCase{
		{
			name:       "force-bendpoint-particles",
			claim:      "Force repulsivePower adds repelling bendpoint particles to an edge",
			upstream:   "ELK 727def679 Added bendpoints to force layout (#889)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("zero", base, semanticElementOption("e1", "elk.force.repulsivePower", "0")),
				semanticVariant("two", base, semanticElementOption("e1", "elk.force.repulsivePower", "2")),
			},
		},
		{
			name:       "force-fixed-graph-size",
			claim:      "Force honors a caller-provided graph width and height when fixedGraphSize is enabled",
			upstream:   "ELK 7acf0b981 add fixedGraphSize support for mrtree and force based layout (#834)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("dynamic", base,
					semanticRootField("width", 420.0), semanticRootField("height", 260.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "false")),
				semanticVariant("fixed", base,
					semanticRootField("width", 420.0), semanticRootField("height", 260.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "true")),
			},
		},
	}
}

// The remaining provider builders are filled out below as independent tables.
// Keeping these small functions separate makes each release-line provenance
// group reviewable without coupling the graph construction helpers.
func mrTreeSemanticCases() []releaseSemanticCase {
	tree := semanticGraph("mrtree", semanticOptions(
		"elk.direction", "RIGHT",
		"elk.spacing.nodeNode", "24",
	), []map[string]any{
		semanticNode("r", 36, 30, nil),
		semanticNode("a", 55, 24, nil),
		semanticNode("b", 28, 46, nil),
		semanticNode("c", 42, 20, nil),
		semanticNode("d", 30, 34, nil),
	}, []map[string]any{
		semanticEdge("ra", "r", "a", nil),
		semanticEdge("rb", "r", "b", nil),
		semanticEdge("ac", "a", "c", nil),
		semanticEdge("ad", "a", "d", nil),
	})

	interactive := semanticGraph("mrtree", semanticOptions(
		"elk.direction", "RIGHT",
	), []map[string]any{
		semanticNode("r", 30, 30, nil),
		semanticNode("a", 30, 30, semanticOptions("elk.mrtree.positionConstraint", "2")),
		semanticNode("b", 30, 30, semanticOptions("elk.mrtree.positionConstraint", "0")),
		semanticNode("c", 30, 30, semanticOptions("elk.mrtree.positionConstraint", "1")),
	}, []map[string]any{
		semanticEdge("ra", "r", "a", nil),
		semanticEdge("rb", "r", "b", nil),
		semanticEdge("rc", "r", "c", nil),
	})
	interactiveEdits := []semanticEdit{
		semanticElementField("r", "x", 20.0), semanticElementField("r", "y", 20.0),
		semanticElementField("a", "x", 260.0), semanticElementField("a", "y", 180.0),
		semanticElementField("b", "x", 100.0), semanticElementField("b", "y", 25.0),
		semanticElementField("c", "x", 180.0), semanticElementField("c", "y", 90.0),
	}

	weighted := semanticGraph("mrtree", semanticOptions(
		"elk.direction", "RIGHT",
	), []map[string]any{
		semanticNode("r", 30, 30, nil),
		semanticNode("small", 30, 30, nil),
		semanticNode("large", 30, 30, nil),
		semanticNode("l1", 30, 30, nil),
		semanticNode("l2", 30, 30, nil),
		semanticNode("l3", 30, 30, nil),
	}, []map[string]any{
		// Edge definition order intentionally puts the leaf before the subtree.
		semanticEdge("r-small", "r", "small", nil),
		semanticEdge("r-large", "r", "large", nil),
		semanticEdge("large-l1", "large", "l1", nil),
		semanticEdge("large-l2", "large", "l2", nil),
		semanticEdge("large-l3", "large", "l3", nil),
	})

	bounds := semanticGraph("mrtree", semanticOptions(
		"elk.padding", "[top=0,left=0,bottom=0,right=0]",
		"elk.spacing.nodeNode", "0",
		"elk.aspectRatio", "1000",
	), []map[string]any{
		semanticNode("a", 15, 30, nil),
		semanticNode("b", 15, 30, nil),
	}, nil)

	return []releaseSemanticCase{
		{
			name:       "mrtree-edge-routing",
			claim:      "MrTree supports center-to-center and overlap-avoiding routing",
			upstream:   "ELK 10f7b345d Interactive MrTree (#884)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("middle-to-middle", tree, semanticRootOption("elk.mrtree.edgeRoutingMode", "MIDDLE_TO_MIDDLE")),
				semanticVariant("avoid-overlap", tree, semanticRootOption("elk.mrtree.edgeRoutingMode", "AVOID_OVERLAP")),
			},
		},
		{
			name:       "mrtree-interactive-order",
			claim:      "MrTree's interactive configuration switches to constraint-weighted node ordering",
			upstream:   "ELK 10f7b345d Interactive MrTree (#884)",
			observable: true,
			variants: []releaseSemanticVariant{
				// InteractiveMrTreeGraphVisitor is a service-side configurator and is not
				// invoked by elkjs merely by setting interactiveLayout. Spell out the
				// two properties it applies so this remains an observable elkjs probe.
				semanticVariant("batch", interactive, append(interactiveEdits,
					semanticRootOption("elk.interactive", "false"),
					semanticRootOption("elk.mrtree.weighting", "MODEL_ORDER"))...),
				semanticVariant("interactive-constraint", interactive, append(interactiveEdits,
					semanticRootOption("elk.interactive", "true"),
					semanticRootOption("elk.mrtree.weighting", "CONSTRAINT"))...),
			},
		},
		{
			name:       "mrtree-model-order-weighting",
			claim:      "MrTree can retain edge-definition model order or weight subtrees by descendants",
			upstream:   "ELK 10f7b345d Interactive MrTree (#884), OrderWeighting",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("model-order", weighted, semanticRootOption("elk.mrtree.weighting", "MODEL_ORDER")),
				semanticVariant("descendants", weighted, semanticRootOption("elk.mrtree.weighting", "DESCENDANTS")),
			},
		},
		{
			name:       "mrtree-fixed-graph-size",
			claim:      "MrTree honors a caller-provided graph width and height when fixedGraphSize is enabled",
			upstream:   "ELK 7acf0b981 add fixedGraphSize support for mrtree and force based layout (#834)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("dynamic", tree,
					semanticRootField("width", 500.0), semanticRootField("height", 320.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "false")),
				semanticVariant("fixed", tree,
					semanticRootField("width", 500.0), semanticRootField("height", 320.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "true")),
			},
		},
		{
			name:       "mrtree-disconnected-component-bounds",
			claim:      "MrTree recomputes graph bounds after horizontally packing disconnected components",
			upstream:   "ELK 9f39b77b5 Recompute graph bounds after packing components in MrTree (#1142)",
			observable: false,
			variants:   []releaseSemanticVariant{semanticVariant("two-components", bounds)},
		},
	}
}
func radialSemanticCases() []releaseSemanticCase {
	base := semanticGraph("radial", semanticOptions(
		"elk.radial.radius", "70",
		"elk.radial.sorter", "NONE",
	), []map[string]any{
		semanticNode("r", 40, 40, nil),
		semanticNode("a", 28, 24, nil),
		semanticNode("b", 45, 20, nil),
		semanticNode("c", 24, 42, nil),
		semanticNode("d", 35, 22, nil),
		semanticNode("e", 30, 30, nil),
	}, []map[string]any{
		semanticEdge("ra", "r", "a", nil),
		semanticEdge("rb", "r", "b", nil),
		semanticEdge("rc", "r", "c", nil),
		semanticEdge("ad", "a", "d", nil),
		semanticEdge("ae", "a", "e", nil),
	})

	return []releaseSemanticCase{
		{
			name:       "radial-rotation",
			claim:      "Radial rotates a completed drawing to a requested target angle",
			upstream:   "ELK 83898117b Radial layout, rotation extensions (#945)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("unrotated", base,
					semanticRootOption("elk.radial.rotate", "false"),
					semanticRootOption("elk.radial.rotation.targetAngle", "1.2")),
				semanticVariant("target-1.2", base,
					semanticRootOption("elk.radial.rotate", "true"),
					semanticRootOption("elk.radial.rotation.targetAngle", "1.2")),
			},
		},
		{
			name:       "radial-node-spacing",
			claim:      "Radial uses half the configured node-node spacing as radial padding",
			upstream:   "ELK c5e9fc210 radial: Correctly use half of the node node spacing as padding (#957)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("zero", base,
					semanticRootOption("elk.radial.radius", "0"),
					semanticRootOption("elk.spacing.nodeNode", "0")),
				semanticVariant("sixty", base,
					semanticRootOption("elk.radial.radius", "0"),
					semanticRootOption("elk.spacing.nodeNode", "60")),
			},
		},
		{
			name:       "radial-center-on-root",
			claim:      "Radial can center the final drawing bounds on the root node",
			upstream:   "ELK e878d4ce6 Radial layout: center on root option (#941)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("compact-bounds", base, semanticRootOption("elk.radial.centerOnRoot", "false")),
				semanticVariant("root-centered", base, semanticRootOption("elk.radial.centerOnRoot", "true")),
			},
		},
	}
}
func rectPackingSemanticCases() []releaseSemanticCase {
	base := semanticGraph("rectpacking", semanticOptions(
		"elk.padding", "[top=10,left=10,bottom=10,right=10]",
		"elk.spacing.nodeNode", "8",
		"elk.aspectRatio", "1.3",
	), []map[string]any{
		semanticNode("a", 74, 18, nil),
		semanticNode("b", 20, 67, nil),
		semanticNode("c", 43, 35, nil),
		semanticNode("d", 83, 16, nil),
		semanticNode("e", 16, 76, nil),
		semanticNode("f", 51, 28, nil),
		semanticNode("g", 29, 49, nil),
	}, nil)

	microA := semanticNode("a", 10, 10, semanticOptions(
		"elk.nodeSize.constraints", "NODE_LABELS",
		"elk.nodeLabels.placement", "INSIDE H_CENTER V_CENTER",
	))
	microA["labels"] = []map[string]any{{"id": "a-label", "text": "wide", "width": 88.0, "height": 18.0}}
	microB := semanticNode("b", 12, 12, semanticOptions(
		"elk.nodeSize.constraints", "NODE_LABELS",
		"elk.nodeLabels.placement", "INSIDE H_CENTER V_CENTER",
	))
	microB["labels"] = []map[string]any{{"id": "b-label", "text": "tall", "width": 24.0, "height": 42.0}}
	micro := semanticGraph("rectpacking", semanticOptions(
		"elk.padding", "[top=4,left=4,bottom=4,right=4]",
	), []map[string]any{microA, microB}, nil)

	return []releaseSemanticCase{
		{
			name:       "rectpacking-width-strategy",
			claim:      "Rectpacking exposes the renamed GREEDY and TARGET_WIDTH approximation strategies",
			upstream:   "ELK 20a09e44c Restructure and improve rectpacking (#865)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("greedy", base, semanticRootOption("elk.rectpacking.widthApproximation.strategy", "GREEDY")),
				semanticVariant("target-130", base,
					semanticRootOption("elk.rectpacking.widthApproximation.strategy", "TARGET_WIDTH"),
					semanticRootOption("elk.rectpacking.widthApproximation.targetWidth", "130")),
			},
		},
		{
			name:       "rectpacking-packing-strategy",
			claim:      "Rectpacking exposes the renamed COMPACTION and SIMPLE packing strategies",
			upstream:   "ELK 20a09e44c Restructure and improve rectpacking (#865)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("compaction", base, semanticRootOption("elk.rectpacking.packing.strategy", "COMPACTION")),
				semanticVariant("simple", base, semanticRootOption("elk.rectpacking.packing.strategy", "SIMPLE")),
			},
		},
		{
			name:       "rectpacking-whitespace-strategy",
			claim:      "Rectpacking exposes explicit NONE and EQUAL_BETWEEN_STRUCTURES whitespace elimination",
			upstream:   "ELK 20a09e44c Restructure and improve rectpacking (#865)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("none", base, semanticRootOption("elk.rectpacking.whiteSpaceElimination.strategy", "NONE")),
				semanticVariant("equal-between", base, semanticRootOption("elk.rectpacking.whiteSpaceElimination.strategy", "EQUAL_BETWEEN_STRUCTURES")),
			},
		},
		{
			name:       "rectpacking-fixed-graph-size",
			claim:      "Rectpacking honors a caller-provided graph width and height when fixedGraphSize is enabled",
			upstream:   "ELK 8439b0e1e rectpacking: Add support for fixed graph size (#855)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("dynamic", base,
					semanticRootField("width", 360.0), semanticRootField("height", 260.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "false")),
				semanticVariant("fixed", base,
					semanticRootField("width", 360.0), semanticRootField("height", 260.0),
					semanticRootOption("elk.nodeSize.fixedGraphSize", "true")),
			},
		},
		{
			name:       "rectpacking-node-micro-layout",
			claim:      "Rectpacking applies common node micro-layout before packing and accounts for labels in node sizes",
			upstream:   "ELK c6f44e8a0 Fix micro layout for rectpacking including content alignment and label placement (#997)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("omit", micro, semanticRootOption("elk.omitNodeMicroLayout", "true")),
				semanticVariant("apply", micro, semanticRootOption("elk.omitNodeMicroLayout", "false")),
			},
		},
		{
			name:       "rectpacking-order-by-size",
			claim:      "Rectpacking can reorder nodes by descending height before packing",
			upstream:   "ELK 37c7dedda and 765676dc8 OrderBySize rectpacking (#1095, #1100)",
			observable: true,
			variants: []releaseSemanticVariant{
				semanticVariant("model-order", base, semanticRootOption("elk.rectpacking.orderBySize", "false")),
				semanticVariant("height-order", base, semanticRootOption("elk.rectpacking.orderBySize", "true")),
			},
		},
	}
}
func layeredSemanticCases() []releaseSemanticCase { return buildLayeredSemanticCases() }
