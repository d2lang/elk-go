# Differential layout corpus

`curated/` contains raw ELK JSON inputs, before any layout coordinates are
produced. The cases cover the graph forms D2 sends to ELK: directions,
branching, cycles, labels, self-loops, compounds, ports, cross-hierarchy edges,
and model-order regressions.

Two inputs retain their external fixture provenance:

| Fixture | Exact source |
| --- | --- |
| `curated/13_moon_issue8_isomorphic.json` | `graph_json_issue8_input_json` in `src/graph/json/issue8_isomorphic_parity_test.mbt` |
| `curated/14_moon_issue9_single_container.json` | `graph_json_issue9_input_json` in `src/graph/json/issue9_single_container_parity_test.mbt` |

Those two fixtures come from
`https://github.com/moonbit-community/moon_elk` at commit
`3761d04fcc42331a39cd2e879a939697327ed6c3`. That pinned repository snapshot
has no tag and is licensed under the Eclipse Public License 2.0 in its root
`LICENSE`. The fixture files contain only the JSON strings returned by the
named MoonBit test helpers; they do not copy the assertions or implementation.

The expected result is intentionally not copied into this repository. Tests run
the inputs against the exact elkjs 0.8.2 asset pinned by SHA-256 in
`internal/oracle` and compare that result with the Go implementation. Set
`ELKJS_ORACLE_ASSET` when the D2 sibling checkout is not available.

The complete D2 E2E extraction is a development corpus kept outside Git because
it duplicates hundreds of project fixtures. Its manifest records the source D2
commit and content hashes so the corpus can be reproduced.

Run the checked-in cases with `ELK_GO_ORACLE_PARITY=1 go test
./internal/oracle -run CandidateCuratedParity`. Run the complete captured D2
corpus by additionally setting `ELK_GO_D2_CORPUS` to the capture directory.
