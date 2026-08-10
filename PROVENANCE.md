# Source provenance

The source target for this native Go port is the exact official ELK.js 0.12.0
release. Its behavioral compatibility contract is narrower: D2's eleven
selectable algorithms at their validated D2-facing entry paths, with complete
end-to-end parity for D2's Layered profile. Much of the implementation
originated as a translation of ELK 0.8.1 and has since been updated selectively
from the ELK 0.12.0 source tag. The historical source map below remains
applicable to unchanged translation units; source-shaped 0.12 additions are
identified in their files and tests. Oracle results complement, but do not
replace, those source mappings and notices.

## Target oracle and source release

- Eclipse Layout Kernel 0.12.0, commit
  `323312916048544e4e594eab3303cdb0d01e2929`.
- ELK.js 0.12.0, commit
  `ff5771d7165445c42c408bb8a090c8035272218c`.
- GWT 2.11.0, commit
  `7f76f0f2b5b1b028d2d870782048e44402637fb0`, as pinned by the exact
  ELK.js 0.12.0 `build.gradle`.
- Node.js 24.19.0, commit
  `cdc1b38d40cb567b7ad0b39c86addf830a0af0ae`, bundling V8
  `13.6.233.17-node.51`. The adapted source file
  `deps/v8/src/base/ieee754.cc` has SHA-256
  `1bf999809d4c4c1d31ad5ff3578d32614796f159e08b9e7782560d017721a0c2`.
- The official ELK.js 0.12.0 `lib/elk.bundled.js` differential oracle has
  SHA-256
  `1222e44f953ce7746af23801e723708f8e6f436b8b377a6a5fc7552f34a307b3`.

ELK 0.12.0 Java sources are the target implementation source. The exact
official ELK.js output is the behavioral oracle where Java-to-JavaScript
compilation or Java runtime details are observable. It exposes eleven
providers; DisCo was removed after 0.8.2 and is outside the target surface.

The two observable GWT runtime adaptations described below were compared
directly between the pinned 2.9.0 and 2.11.0 tags. `PriorityQueue.java` is
unchanged, and the object-array `Arrays.sort` legacy merge-sort routines are
unchanged; the 2.11.0 `Arrays.java` changes are outside that object-sort path.

## Existing-port baseline releases

- Eclipse Layout Kernel 0.8.1, commit
  `5dc0f65c78778beb74a36e0f470c6fe6ecdcef67`.
- ELK.js 0.8.2, commit
  `cd0dc6b4e4bf1f221db04147f88b4d7c7041b834`.
- GWT 2.9.0, commit
  `8188fd9f8e1b70396ba02ce01b1d73a467225a64`.
- MoonBit Community `moon_elk`, unreleased repository snapshot (no tag),
  commit `3761d04fcc42331a39cd2e879a939697327ed6c3`.
- The frozen D2 ELK.js bundle formerly used by the differential tests has
  SHA-256
  `a297b92da4a257b95c129cc1d8cfb379e2c89bcd4442af90ac06fdd2b1e220b4`.

The ELK 0.8.1 Java sources remain the historical implementation provenance for
unchanged Go files. The frozen 0.8.2 bundle is historical evidence only and is
no longer accepted by the differential runner.

Two curated input-only fixtures are copied from the pinned `moon_elk` test
suite. The source repository is
`https://github.com/moonbit-community/moon_elk`; its root `LICENSE` applies the
Eclipse Public License 2.0. The pinned commit has no tag. The exact mapping is:

| Elk-go fixture | `moon_elk` source at `3761d04fcc42331a39cd2e879a939697327ed6c3` |
| --- | --- |
| `testdata/curated/13_moon_issue8_isomorphic.json` | JSON returned by `graph_json_issue8_input_json` in `src/graph/json/issue8_isomorphic_parity_test.mbt` |
| `testdata/curated/14_moon_issue9_single_container.json` | JSON returned by `graph_json_issue9_input_json` in `src/graph/json/issue9_single_container_parity_test.mbt` |

## Port map

The following Go packages originated as translations and adaptations of the
listed ELK 0.8.1 source areas. Later 0.12 source-shaped changes retain that
baseline mapping and identify their newer source in file comments or tests.
Applicable upstream copyright notices are preserved in the translated files.

| Go package | ELK 0.8.1 source area |
| --- | --- |
| `internal/geometry` | `org.eclipse.elk.core.math` and core spacing/padding value types |
| `internal/common/networksimplex` | `org.eclipse.elk.alg.common.networksimplex` |
| `internal/common/nodespacing` | `org.eclipse.elk.alg.common.nodespacing` |
| `internal/basic`, `internal/fixed`, `internal/box`, `internal/random` | ELK core Fixed, Box, and Random layout providers and their generated option metadata |
| `internal/force` | `org.eclipse.elk.alg.force` |
| `internal/stress` | `org.eclipse.elk.alg.force.stress` |
| `internal/mrtree` | `org.eclipse.elk.alg.mrtree` |
| `internal/radial` | `org.eclipse.elk.alg.radial` |
| `internal/spore` | `org.eclipse.elk.alg.spore`, including overlap removal, Delaunay structure extraction, spanning-tree processing, and compaction |
| `internal/rectpacking` | `org.eclipse.elk.alg.rectpacking`, including both approximation and row/block compaction passes |
| `internal/layered` | `org.eclipse.elk.alg.layered`, including graph transforms, compound layout, and options |
| `internal/layered/p1cycles` | `org.eclipse.elk.alg.layered.p1cycles` |
| `internal/layered/p2layers` | `org.eclipse.elk.alg.layered.p2layers` |
| `internal/layered/p3order` | `org.eclipse.elk.alg.layered.p3order` and its crossing-counting helpers |
| `internal/layered/p4nodes` | `org.eclipse.elk.alg.layered.p4nodes` and Brandes-Koepf helpers |
| `internal/layered/p5edges` | `org.eclipse.elk.alg.layered.p5edges`, including orthogonal routing |
| `internal/layered/intermediate` | `org.eclipse.elk.alg.layered.intermediate` and its self-loop helpers |
| `internal/layered/components` | `org.eclipse.elk.alg.layered.components` and ELK component packing behavior |
| `internal/layeredlayout` | the ELK Layered provider and processor schedule |

For one-class translations, the Go filename, exported type or processor
comment names the corresponding class in the source area above (for example,
`p2layers/coffman_graham.go` maps to `CoffmanGrahamLayerer.java`). Go tests
that adapt a named Java test map to the same path below `test/` rather than to
the production class. The non-obvious and multi-source mappings are listed
explicitly here; these are also the files whose headers must preserve more
than one distinct upstream notice:

| Go file | Exact ELK 0.8.1 source file(s) |
| --- | --- |
| `internal/common/networksimplex/n_graph.go` | `plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/NGraph.java`, `NNode.java`, and `NEdge.java` in the same directory |
| `internal/common/nodespacing/node_label_and_size_calculator.go` | `plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/NodeDimensionCalculation.java` and `NodeLabelAndSizeCalculator.java` |
| `internal/common/nodespacing/options.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk` and the `NodeLabelPlacement.java`, `PortLabelPlacement.java`, `EdgeLabelPlacement.java`, `SizeOptions.java`, and `PortAlignment.java` files in its `options/` directory |
| `internal/basicoptions/options.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk`, `options/EdgeRouting.java`, and `options/SizeOptions.java` |
| `internal/box/provider.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/BoxLayoutProvider.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk` |
| `internal/fixed/provider.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/FixedLayoutProvider.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk` |
| `internal/random/provider.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/RandomLayoutProvider.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk` |
| `internal/builtins/frozen_options.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/IndividualSpacings.java` for `parseIndividualSpacings`; the remaining compatibility metadata is transcribed from `knownLayoutOptions()` output of the pinned ELK.js asset rather than from an additional implementation source file |
| `internal/core/codec.go` | `plugins/org.eclipse.elk.graph.json/src/org/eclipse/elk/graph/json/JsonImporter.xtend` and `JsonExporter.xtend` |
| `internal/core/engine.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/RecursiveGraphLayoutEngine.java` and `data/LayoutAlgorithmResolver.java` |
| `internal/core/options.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk`, `util/IndividualSpacings.java`, and `options/Direction.java`, `HierarchyHandling.java`, `ContentAlignment.java`, `PortSide.java`, `PortConstraints.java`, `SizeConstraint.java`, `PortAlignment.java`, and `EdgeType.java` |
| `internal/core/registry.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/Core.melk` and `plugins/org.eclipse.elk.graph/src/org/eclipse/elk/graph/properties/GraphFeature.java` |
| `internal/geometry/geometry.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/math/KVector.java`, `KVectorChain.java`, `ElkRectangle.java`, `ElkPadding.java`, and `ElkMargin.java` |
| `internal/graph/model.go` | `plugins/org.eclipse.elk.graph/model/elkgraph.genmodel` and its generated `ElkNode.java`, `ElkPort.java`, `ElkEdge.java`, `ElkEdgeSection.java`, `ElkLabel.java`, `ElkShape.java`, and `ElkBendPoint.java` model interfaces |
| `internal/graph/properties.go` | `plugins/org.eclipse.elk.graph/src/org/eclipse/elk/graph/properties/IProperty.java`, `Property.java`, `IPropertyHolder.java`, and `MapPropertyHolder.java` |
| `internal/force/graph.go` | the `FGraph.java`, `FNode.java`, `FEdge.java`, `FParticle.java`, `FBendpoint.java`, and `FLabel.java` files below `plugins/org.eclipse.elk.alg.force/src/org/eclipse/elk/alg/force/graph/` |
| `internal/force/importer.go` | `plugins/org.eclipse.elk.alg.force/src/org/eclipse/elk/alg/force/ElkGraphImporter.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java` |
| `internal/force/provider.go` | `plugins/org.eclipse.elk.alg.force/src/org/eclipse/elk/alg/force/ForceLayoutProvider.java` and `Force.melk` in the same directory |
| `internal/layered/compound.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CompoundGraphPreprocessor.java`, `CompoundGraphPostprocessor.java`, `CrossHierarchyEdge.java`, and `CrossHierarchyEdgeComparator.java` |
| `internal/layered/graph.go` | the `LGraph.java`, `LNode.java`, `LPort.java`, `LEdge.java`, `LLabel.java`, and `Layer.java` files below `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/`, plus `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/options/PortSide.java` |
| `internal/layered/importer.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphImporter.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/IndividualSpacings.java` |
| `internal/layered/internal_properties.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/options/InternalProperties.java`, `intermediate/compaction/HorizontalGraphCompactor.java`, and `p5edges/splines/SplineSegment.java` |
| `internal/layered/lgraph_util.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/LGraphUtil.java` and `LEdge.java` |
| `internal/layered/options.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/Layered.melk`, each Java enum in its `options/` directory for which this file declares the corresponding Go type, the core `Direction.java`, `HierarchyHandling.java`, `PortConstraints.java`, `EdgeRouting.java`, `Alignment.java`, `EdgeLabelPlacement.java`, and `LabelSide.java` enums, and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/IndividualSpacings.java` |
| `internal/layered/options_registry.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/Layered.melk`, the enum sources mapped for `options.go`, and the core `NodeLabelPlacement.java`, `PortLabelPlacement.java`, `SizeConstraint.java`, `SizeOptions.java`, and `PortAlignment.java` sources whose values are reproduced in the registry |
| `internal/layered/phases.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/LayeredPhases.java`, `intermediate/IntermediateProcessorStrategy.java`, and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/alg/LayoutProcessorConfiguration.java` |
| `internal/layered/properties.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/LGraphElement.java` and `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/Layered.melk` |
| `internal/layered/spacings.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/options/Spacings.java` and `LayeredSpacings.java` |
| `internal/layered/transferrer.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphLayoutTransferrer.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java` |
| `internal/layered/components/components_processor.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/components/ComponentsProcessor.java`, `ComponentOrderingStrategy.java`, and `../ElkLayered.java` |
| `internal/layered/intermediate/comments.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/CommentPreprocessor.java`, `CommentNodeMarginCalculator.java`, and `CommentPostprocessor.java` |
| `internal/layered/intermediate/end_labels.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/EndLabelPreprocessor.java`, `EndLabelSorter.java`, and `EndLabelPostprocessor.java`; `plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/LabelCell.java`, `HorizontalLabelAlignment.java`, and `VerticalLabelAlignment.java`; and `plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/overlaps/RectangleStripOverlapRemover.java` |
| `internal/layered/intermediate/hierarchical_port_router.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/HierarchicalPortOrthogonalEdgeRouter.java` |
| `internal/layered/intermediate/hierarchical_ports.go` | the `HierarchicalPortConstraintProcessor.java`, `HierarchicalPortDummySizeProcessor.java`, `HierarchicalPortPositionProcessor.java`, and `HierarchicalNodeResizingProcessor.java` files in that same `intermediate/` directory |
| `internal/layered/intermediate/label_and_node_size.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/LabelAndNodeSizeProcessor.java` and `plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/NodeLabelAndSizeCalculator.java` |
| `internal/layered/intermediate/post_compaction.go` | the one-dimensional graph and algorithm classes below `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compaction/oned/`, plus `intermediate/compaction/HorizontalGraphCompactor.java`, `LGraphToCGraphTransformer.java`, `VerticalSegment.java`, `EdgeAwareScanlineConstraintCalculation.java`, and `NetworkSimplexCompaction.java` |
| `internal/layered/p3order/port_distributor.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/AbstractBarycenterPortDistributor.java`, `NodeRelativePortDistributor.java`, and `LayerTotalPortDistributor.java` |
| `internal/layered/p5edges/orthogonal/direction.go` | the `BaseRoutingDirectionStrategy.java`, `NorthToSouthRoutingStrategy.java`, `SouthToNorthRoutingStrategy.java`, `WestToEastRoutingStrategy.java`, and `RoutingDirection.java` files below `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/direction/` |
| `internal/layered/p5edges/spline_edge_router.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/splines/SplineEdgeRouter.java`, `SplineSegment.java`, `NubSpline.java`, and `SplinesMath.java`, plus `intermediate/FinalSplineBendpointsCalculator.java` |
| `internal/layeredlayout/provider.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/ElkLayered.java`, `LayeredLayoutProvider.java`, `GraphConfigurator.java`, and `Layered.melk` |
| `internal/mrtree/provider.go` | `plugins/org.eclipse.elk.alg.mrtree/src/org/eclipse/elk/alg/mrtree/Tree.melk`, `TreeLayoutProvider.java`, `MrTree.java`, `TreeLayoutPhases.java`, `TreeUtil.java`, `ElkGraphImporter.java`, `ComponentsProcessor.java`, `p1treeify/DFSTreeifyer.java`, `p2order/NodeOrderer.java`, `p3place/NodePlacer.java`, `p4route/EdgeRouter.java`, and `intermediate/IntermediateProcessorStrategy.java`, `RootProcessor.java`, `FanProcessor.java`, `LevelHeightProcessor.java`, `NeighborsProcessor.java`, and `NodePositionProcessor.java` |
| `internal/radial/provider.go` | `plugins/org.eclipse.elk.alg.radial/src/org/eclipse/elk/alg/radial/Radial.melk`, `RadialLayoutProvider.java`, `RadialLayoutPhases.java`, `RadialUtil.java`; `p1position/EadesRadial.java`, `p1position/wedge/AnnulusWedgeByLeafs.java`, `AnnulusWedgeByNodeSpace.java`; `sorting/IDSorter.java`, `PolarCoordinateSorter.java`; `intermediate/CalculateGraphSize.java`; `intermediate/optimization/CrossingMinimizationPosition.java`, `EdgeLengthOptimization.java`, `EdgeLengthPositionOptimization.java`; `intermediate/overlaps/RadiusExtensionOverlapRemoval.java`; `intermediate/compaction/AbstractRadiusExtensionCompaction.java`, `AnnulusWedgeCompaction.java`, `GeneralCompactor.java`, `RadialCompaction.java`; and `p2routing/StraightLineEdgeRouter.java` |
| `internal/rectpacking/provider.go` | `plugins/org.eclipse.elk.alg.rectpacking/src/org/eclipse/elk/alg/rectpacking/RectPacking.melk`, `RectPackingLayoutProvider.java`, `InteractiveRectPackingGraphVisitor.java`; `firstiteration/AreaApproximation.java`, `AreaFilter.java`, `AspectRatioFilter.java`, `BestCandidateFilter.java`, `Calculations.java`, `ScaleMeasureFilter.java`; and `util/DrawingData.java` |
| `internal/rectpacking/second_iteration.go` | the second-iteration classes below `plugins/org.eclipse.elk.alg.rectpacking/src/org/eclipse/elk/alg/rectpacking/seconditeration/` and `util/Block.java`, `BlockRow.java`, and `BlockStack.java` |

### ELK 0.12 source-shaped additions

The following files implement behavior added or materially changed after the
historical 0.8.1 baseline. Paths in this table resolve against the exact ELK
0.12.0 commit pinned above.

| Go file | Exact ELK 0.12.0 source file(s) |
| --- | --- |
| `internal/core/codec.go` | `plugins/org.eclipse.elk.graph.json/src/org/eclipse/elk/graph/json/JsonImporter.xtend`, `JsonExporter.xtend`, and `JsonAdapter.xtend`; `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/data/LayoutOptionData.java`; and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/options/ShapeCoords.java` and `EdgeCoords.java` |
| `internal/core/topdown.go` | `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/RecursiveGraphLayoutEngine.java` and `plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java` (`computeChildAreaDimensions`) |
| `internal/layered/compound.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CompoundGraphPostprocessor.java` and `graph/LGraphUtil.java` (`changeCoordSystem` identity and coordinate-operation order) |
| `internal/layered/transferrer.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/transform/ElkGraphLayoutTransferrer.java` (anchor, hierarchy offset, bend-point, and edge-label coordinate grouping) |
| `internal/layered/intermediate/node_placement_repeater.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/NodePlacementRepeater.java`, together with the existing Go translations of `LabelAndNodeSizeProcessor.java` and `NodeMarginCalculator.java` that it invokes |
| `internal/layered/intermediate/preserveorder/cm_group_model_order.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/preserveorder/CMGroupModelOrderCalculator.java` |
| `internal/layered/intermediate/loops/ordering/port_restorer.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/ordering/PortRestorer.java` and `options/SelfLoopOrderingStrategy.java` (SEQUENCED hidden-port orientation and REVERSE_STACKED) |
| `internal/layered/intermediate/wrapping/breaking_point_inserter.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/wrapping/BreakingPointInserter.java`, including the MANUAL-cut and split-edge model-order corrections from ELK commit `c9f0d4c1e3b2c0584ea5603b2627f7a53c9b7eb2` |
| `internal/layered/intermediate/unzipping/alternating_layer_unzipper.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/unzipping/AlternatingLayerUnzipper.java` |
| `internal/layered/p1cycles/dfs_node_order.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/DFSNodeOrderCycleBreaker.java` |
| `internal/layered/p1cycles/scc_node_type.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/SCCNodeTypeCycleBreaker.java` |
| `internal/layered/p5edges/spline_edge_router.go` | `plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/splines/SplineEdgeRouter.java` and `intermediate/FinalSplineBendpointsCalculator.java` (CONSERVATIVE_SOFT routing split) |

The source-shaped Go tests use the same production sources as the file under
test unless their header comes from an ELK test class. Non-obvious test
mappings are:

| Go test | Exact ELK 0.8.1 test source |
| --- | --- |
| `internal/layered/intermediate/loops/routing/polyline_router_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/intermediate/loops/routing/PolylineSelfLoopRouterTest.java` |
| `internal/layered/p3order/all_crossings_counter_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/p3order/counting/AllCrossingsCounterTest.java`, in addition to the production counter |
| `internal/layered/p3order/barycenter_heuristic_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/p3order/BarycenterHeuristicTest.java`, in addition to the production heuristic |
| `internal/layered/p3order/crossings_counter_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/p3order/counting/CrossingsCounterTest.java`, in addition to the production counter |
| `internal/layered/p3order/model_order_barycenter_heuristic_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/p3order/ModelOrderBarycenterHeuristicTest.java`, in addition to the production heuristic |
| `internal/layered/p4nodes/network_simplex_test.go` | `test/org.eclipse.elk.alg.layered.test/src/org/eclipse/elk/alg/layered/p4nodes/BasicNodePlacementTest.java` |

Two observable JavaScript-runtime routines are separately adapted from GWT
2.11.0 and retain its Apache License 2.0. Their relevant implementations and
the checked-in behavior vectors are unchanged from the GWT 2.9.0 baseline:

| Go file | GWT 2.11.0 source |
| --- | --- |
| `internal/javacompat/gwt_priority_queue.go` | `user/super/com/google/gwt/emul/java/util/PriorityQueue.java` |
| `internal/javacompat/gwt_stable_sort.go` | the legacy merge-sort path in `user/super/com/google/gwt/emul/java/util/Arrays.java` |

`internal/javacompat/jsmath.go` adapts the sine, cosine, argument-reduction,
and kernel routines from the exact V8 source pinned above. It retains V8's BSD
3-Clause license and the embedded fdlibm Sun Microsystems permission notice;
see `LICENSE.v8-bsd` and the file header.

Additional algorithm packages identify their corresponding ELK source areas
in their package documentation and file headers.

## Elk-go-authored infrastructure

The public Go API, ordered graph arenas, JSON codec, registry/engine glue,
command-line tools, differential harness, and Go tests under `graphjson`,
`internal/graph`, `internal/core`, `internal/oracle`, and `cmd` are elk-go
adaptations or original work written for this port.

Except for the explicitly attributed GWT and V8/fdlibm adaptations above,
`internal/javacompat` is compatibility code implementing observable Java/GWT
behavior from specifications and independently generated behavior vectors. It
does not contain OpenJDK code. The project did not copy code from the
third-party Rust project `elkrs`; ELK 0.8.1 and 0.12.0 Java, GWT 2.11.0 where
identified (with 2.9.0 retained as historical provenance), and the pinned
ELK.js artifacts remain the implementation and behavioral sources.

## Licensing

ELK 0.8.1, ELK 0.12.0, ELK.js 0.8.2, ELK.js 0.12.0, and the pinned `moon_elk`
snapshot are licensed under the Eclipse Public License 2.0. ELK 0.12.0 and
ELK.js 0.12.0 additionally state the GPL-3.0-or-later secondary-license option;
this project continues to distribute the port under EPL-2.0. Elk-go is
distributed under the same primary license, except for the two attributed GWT
runtime adaptations distributed under Apache License 2.0 and the attributed
V8/fdlibm compatibility routines distributed under BSD-3-Clause and SunPro
terms. See `LICENSE`, `LICENSE.apache-2.0`, `LICENSE.v8-bsd`, `NOTICE.md`, and
the content-preserving copy of the upstream project notice in
`NOTICE.eclipse-elk.md`.
