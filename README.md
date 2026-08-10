# elk-go

`elk-go` is a native Go port of the Eclipse Layout Kernel behavior used by D2.
Its compatibility target is D2's ELK integration running against the exact
official ELK.js 0.12.0 release; it is not a general drop-in replacement for
every ELK option combination.

The port is based on:

- [ELK.js 0.12.0](https://github.com/kieler/elkjs/tree/0.12.0), commit
  `ff5771d7165445c42c408bb8a090c8035272218c`.
- [Eclipse Layout Kernel 0.12.0](https://github.com/eclipse-elk/elk/tree/v0.12.0),
  commit `323312916048544e4e594eab3303cdb0d01e2929`.

ELK.js compiles ELK's Java algorithms to JavaScript with GWT. This repository
translates the provider registry used by D2 to Go while preserving the
ordering, option-default, and floating-point behavior observable on that path.
It registers D2's 11 selectable 0.12 algorithms: Fixed, Box, Random, Layered,
Stress, MR Tree, Radial, Force, SPOrE Overlap, SPOrE Compaction, and Rectangle
Packing. ELK.js removed DisCo from its bundled algorithm surface after 0.8.2,
so it is not registered by this version.

D2's default and documented engine is Layered. Its complete D2 option profile,
including compounds, ports, self loops, labels, model order, and orthogonal
routing, is the primary end-to-end compatibility contract. Exact,
deterministic oracle fixtures cover all eleven registered providers. D2 also
smoke-tests its literal provider defaults; unseeded Random and the two SPOrE
providers use finite-output checks because those default calls are
nondeterministic.

The contract tests vary algorithm selection, all four directions,
`nodeNodeBetweenLayers`, padding, `edgeNodeBetweenLayers`, and `nodeSelfLoop`,
and include compound graphs, ports, inline edge labels, and self loops. Each
scalar option pair is also required to change official-oracle geometry, which
guards against a silently ignored option.

The option metadata exposed by the Go registry matches the official 0.12
bundle so D2 can resolve its option IDs consistently. Metadata presence is not
an execution guarantee for expert ELK configurations outside D2's profile.
Known out-of-scope Layered values include `SCC_CONNECTIVITY` and
`BFS_NODE_ORDER` cycle breaking; `LONGEST_PATH_SOURCE`, `BF_MODEL_ORDER`, and
`DF_MODEL_ORDER` layering; and `MEDIAN_LAYER_SWEEP` crossing minimization.

## API

The public graph types mirror ELK JSON. Layout can be requested either with a
typed graph or with JSON:

```go
graph := &elk.Graph{
    ID: elk.StringID("root"),
    LayoutOptions: elk.LayoutOptions{
        "elk.algorithm": "layered",
        "elk.direction": "DOWN",
    },
    Children: []*elk.Node{
        {ID: elk.StringID("a"), Width: elk.Float64(80), Height: elk.Float64(40)},
        {ID: elk.StringID("b"), Width: elk.Float64(80), Height: elk.Float64(40)},
    },
    Edges: []*elk.Edge{
        {ID: elk.StringID("e"), Sources: []elk.ID{elk.StringID("a")}, Targets: []elk.ID{elk.StringID("b")}},
    },
}

if err := elk.Layout(graph); err != nil {
    // handle the layout error
}
```

## Compatibility testing

Normal tests are pure Go:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Development differentials compare ordered ELK JSON output with the official
ELK.js 0.12.0 oracle. IDs, element order, topology, edge-section structure,
field presence, and coordinates are exact for the provider matrix, curated D2
fixtures, D2-profile random cases, codec behavior used by D2, and the complete
captured D2 corpus. Broader provider and expert-option probes are development
diagnostics, not part of the supported D2 contract. See `internal/oracle` for
the opt-in oracle setup.

## License

This program is available under the Eclipse Public License 2.0. Two attributed
GWT runtime adaptations retain the Apache License 2.0; the V8/fdlibm math
adaptation retains its BSD-3-Clause and SunPro terms. See
[`LICENSE`](LICENSE), [`LICENSE.apache-2.0`](LICENSE.apache-2.0),
[`LICENSE.v8-bsd`](LICENSE.v8-bsd),
[`NOTICE.md`](NOTICE.md), the preserved
[`NOTICE.eclipse-elk.md`](NOTICE.eclipse-elk.md), and
[`PROVENANCE.md`](PROVENANCE.md).
