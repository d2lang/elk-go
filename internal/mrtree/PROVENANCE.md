# ELK Mr. Tree provenance

This directory is a Go translation of Eclipse ELK commit
`5dc0f65c78778beb74a36e0f470c6fe6ecdcef67` (ELK 0.8.1). The Java sources are
licensed under EPL-2.0.

| Go file | ELK 0.8.1 source |
| --- | --- |
| `options.go` | `Tree.melk`, `options/OrderWeighting.java`, and `options/TreeifyingOrder.java`; the option registry values generated upstream from `Tree.melk` are represented directly in Go |
| `model.go` | `graph/TGraphElement.java`, `graph/TShape.java`, `graph/TGraph.java`, `graph/TNode.java`, `graph/TEdge.java` |
| `provider.go` | `Tree.melk`, `TreeLayoutProvider.java`, `ElkGraphImporter.java`, `ComponentsProcessor.java`, `MrTree.java`, `TreeLayoutPhases.java`, `TreeUtil.java`, `p1treeify/DFSTreeifyer.java`, `p2order/NodeOrderer.java`, `p3place/NodePlacer.java`, `p4route/EdgeRouter.java`, and `intermediate/IntermediateProcessorStrategy.java`, `RootProcessor.java`, `FanProcessor.java`, `LevelHeightProcessor.java`, `NeighborsProcessor.java`, and `NodePositionProcessor.java` |
| `provider_test.go` | Source-shaped tests derived from `NodePlacer.java`, `NodeOrderer.java`, and `Tree.melk` |
| `provider_oracle_test.go` | Differential fixtures now verified against pinned official elkjs 0.12.0; historical implementation expectations originated with the 0.8.2 bundle |

No code in this directory is copied from elkrs or a newer ELK release.
