# elk-go

`elk-go` is a native Go port of the Eclipse Layout Kernel algorithms exposed by
ELK.js 0.8.2 and used by D2. Its compatibility target is the exact ELK.js
version embedded by D2, not the newest ELK release.

The port is based on:

- [ELK.js 0.8.2](https://github.com/kieler/elkjs/tree/0.8.2), commit
  `cd0dc6b4e4bf1f221db04147f88b4d7c7041b834`.
- [Eclipse Layout Kernel 0.8.1](https://github.com/eclipse-elk/elk/tree/v0.8.1),
  commit `5dc0f65c78778beb74a36e0f470c6fe6ecdcef67`.

ELK.js compiles ELK's Java algorithms to JavaScript with GWT. This repository
translates the provider registry used by D2 to Go while preserving observable
ordering, Java/GWT runtime, option-default, and floating-point behavior. It
registers the same 12 algorithms as the frozen bundle: Fixed, Box, Random,
Layered, Stress, MR Tree, Radial, Force, DisCo, SPOrE Overlap, SPOrE
Compaction, and Rectangle Packing.

D2's default and documented engine is Layered. Its complete D2 option profile,
including compounds, ports, self loops, labels, model order, and orthogonal
routing, is the primary end-to-end compatibility contract. Provider-specific
tests additionally pin the other registered algorithms and their supported
options to ELK.js 0.8.2.

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

Development differentials compare ordered ELK JSON output with the frozen
ELK.js 0.8.2 oracle. IDs, element order, topology, edge-section structure, and
field presence are exact; only finite layout coordinates use a narrow numeric
tolerance. The checked-in gates include a D2-shaped all-provider matrix; the
full D2 extraction and deterministic randomized cases are maintained as
development corpora. See `internal/oracle` for the opt-in oracle setup.

## License

This program is available under the Eclipse Public License 2.0. Two attributed
GWT runtime adaptations retain the Apache License 2.0. See
[`LICENSE`](LICENSE), [`LICENSE.apache-2.0`](LICENSE.apache-2.0),
[`NOTICE.md`](NOTICE.md), the preserved
[`NOTICE.eclipse-elk.md`](NOTICE.eclipse-elk.md), and
[`PROVENANCE.md`](PROVENANCE.md).
