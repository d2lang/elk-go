# ELK 0.12 native-port differential map

## Exact oracle pin

- elkjs release: `0.12.0`
- elkjs tag commit: `ff5771d7165445c42c408bb8a090c8035272218c`
- official `lib/elk.bundled.js` SHA-256:
  `1222e44f953ce7746af23801e723708f8e6f436b8b377a6a5fc7552f34a307b3`
- corresponding Eclipse ELK tag: `v0.12.0`, commit
  `323312916048544e4e594eab3303cdb0d01e2929`

The oracle rejects all other JavaScript artifacts. Expected layout JSON is not
checked in; each differential obtains it from the pinned bundle at test time.

## Algorithm surface

ELK.js 0.12.0 exposes eleven algorithms in this exact order:

1. `org.eclipse.elk.fixed`
2. `org.eclipse.elk.box`
3. `org.eclipse.elk.random`
4. `org.eclipse.elk.layered`
5. `org.eclipse.elk.stress`
6. `org.eclipse.elk.mrtree`
7. `org.eclipse.elk.radial`
8. `org.eclipse.elk.force`
9. `org.eclipse.elk.sporeOverlap`
10. `org.eclipse.elk.sporeCompaction`
11. `org.eclipse.elk.rectpacking`

Compared with the former 0.8.2 oracle, no provider was added and
`org.eclipse.elk.disco` was removed. DisCo and its Polyomino-only metadata must
not remain in the completed 0.12 registry.

## Supported compatibility boundary

The release contract is the behavior D2 can select:

- all eleven registered algorithms have exact deterministic official-0.12
  fixtures; D2 separately smoke-tests literal provider defaults, using
  finite-output checks for unseeded Random and the two nondeterministic SPOrE
  defaults;
- Layered has exact end-to-end coverage for D2's adapter profile, including
  compounds, ports, labels, self loops, model order, directions, and routing;
- `TestD2ContractExposedOptionMatrixParity` varies
  `nodeNodeBetweenLayers`, padding, `edgeNodeBetweenLayers`, and
  `nodeSelfLoop`, requires exact output for both values, and verifies that the
  official geometry actually changes;
- `TestCandidateProviderMatrixParity` varies all eleven algorithm IDs, while
  `TestD2ContractCuratedCoverage` pins all four directions plus compound,
  ports, inline-label, and self-loop fixtures;
- the captured D2 corpus, D2-profile random cases, and curated D2 fixtures are
  strict, bit-exact gates.

This is not a claim that elk-go executes every expert ELK 0.12 option
combination. In particular, these registered Layered enum values are outside
the D2 contract and are not implemented: cycle breaking `SCC_CONNECTIVITY` and
`BFS_NODE_ORDER`; layering `LONGEST_PATH_SOURCE`, `BF_MODEL_ORDER`, and
`DF_MODEL_ORDER`; and crossing minimization `MEDIAN_LAYER_SWEEP`. Unsupported
phase selections return an error instead of silently using another strategy.

## Known-layout-option metadata (not a support contract)

`knownLayoutOptions()` grows from 198 entries in 0.8.2 to 235 in 0.12.0. No
shared entry changes its ID, group, type, or targets. There are 51 additions
and 14 removals. Elk-go mirrors this metadata so D2's option lookup remains
compatible; registering an option does not by itself put every value or option
combination inside the executable support boundary. Enum-value additions to
pre-existing option IDs are also not counted in the 51 additions below.

Added in 0.12.0:

- `org.eclipse.elk.childAreaHeight`
- `org.eclipse.elk.childAreaWidth`
- `org.eclipse.elk.json.edgeCoords`
- `org.eclipse.elk.json.shapeCoords`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.componentGroupId`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.crossingMinimizationId`
- `org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId`
- `org.eclipse.elk.layered.considerModelOrder.portModelOrder`
- `org.eclipse.elk.layered.crossingMinimization.inLayerPredOf`
- `org.eclipse.elk.layered.crossingMinimization.inLayerSuccOf`
- `org.eclipse.elk.layered.layerUnzipping.layerSplit`
- `org.eclipse.elk.layered.layerUnzipping.minimizeEdgeLength`
- `org.eclipse.elk.layered.layerUnzipping.resetOnLongEdges`
- `org.eclipse.elk.layered.layerUnzipping.strategy`
- `org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement`
- `org.eclipse.elk.mrtree.compaction`
- `org.eclipse.elk.mrtree.edgeEndTextureLength`
- `org.eclipse.elk.mrtree.edgeRoutingMode`
- `org.eclipse.elk.mrtree.positionConstraint`
- `org.eclipse.elk.mrtree.treeLevel`
- `org.eclipse.elk.radial.centerOnRoot`
- `org.eclipse.elk.radial.rotate`
- `org.eclipse.elk.radial.rotation.computeAdditionalWedgeSpace`
- `org.eclipse.elk.radial.rotation.outgoingEdgeAngles`
- `org.eclipse.elk.radial.rotation.targetAngle`
- `org.eclipse.elk.rectpacking.inNewRow`
- `org.eclipse.elk.rectpacking.orderBySize`
- `org.eclipse.elk.rectpacking.packing.compaction.iterations`
- `org.eclipse.elk.rectpacking.packing.compaction.rowHeightReevaluation`
- `org.eclipse.elk.rectpacking.packing.strategy`
- `org.eclipse.elk.rectpacking.trybox`
- `org.eclipse.elk.rectpacking.whiteSpaceElimination.strategy`
- `org.eclipse.elk.rectpacking.widthApproximation.lastPlaceShift`
- `org.eclipse.elk.rectpacking.widthApproximation.optimizationGoal`
- `org.eclipse.elk.rectpacking.widthApproximation.strategy`
- `org.eclipse.elk.rectpacking.widthApproximation.targetWidth`
- `org.eclipse.elk.softwrappingFuzziness`
- `org.eclipse.elk.topdown.hierarchicalNodeAspectRatio`
- `org.eclipse.elk.topdown.hierarchicalNodeWidth`
- `org.eclipse.elk.topdown.nodeType`
- `org.eclipse.elk.topdown.scaleCap`
- `org.eclipse.elk.topdown.scaleFactor`
- `org.eclipse.elk.topdown.sizeApproximator`
- `org.eclipse.elk.topdown.sizeCategories`
- `org.eclipse.elk.topdown.sizeCategoriesHierarchicalNodeWeight`
- `org.eclipse.elk.topdownLayout`

Removed in 0.12.0:

- `org.eclipse.elk.disco.componentCompaction.componentLayoutAlgorithm`
- `org.eclipse.elk.disco.componentCompaction.strategy`
- `org.eclipse.elk.disco.debug.discoGraph`
- `org.eclipse.elk.disco.debug.discoPolys`
- `org.eclipse.elk.polyomino.fill`
- `org.eclipse.elk.polyomino.highLevelSort`
- `org.eclipse.elk.polyomino.lowLevelSort`
- `org.eclipse.elk.polyomino.traversalStrategy`
- `org.eclipse.elk.rectpacking.expandToAspectRatio`
- `org.eclipse.elk.rectpacking.lastPlaceShift`
- `org.eclipse.elk.rectpacking.onlyFirstIteration`
- `org.eclipse.elk.rectpacking.optimizationGoal`
- `org.eclipse.elk.rectpacking.rowCompaction`
- `org.eclipse.elk.rectpacking.targetWidth`

`TestReleaseOptionAuditCoverage` is the machine-checkable source of truth for
this metadata delta. It requires exactly 51 unique additions, exactly 14
sorted removals, registration of every addition, absence of every removal,
and an evidence reference for every added option. This supplemental audit does
not expand the D2 support contract. The 51 additions divide into these test
classes:

- 39 have targeted observable JSON differentials.
- 10 are official `LayoutJSON` no-ops or computed/service-only state:
  `childAreaHeight`, `childAreaWidth`, `softwrappingFuzziness`,
  `topdown.scaleFactor`, `topdown.sizeCategories`,
  `topdown.sizeCategoriesHierarchicalNodeWeight`, `mrtree.treeLevel`,
  `groupModelOrder.componentGroupId`, `inLayerPredOf`, and `inLayerSuccOf`.
- 2 are advertised in metadata but rejected by the official generated JSON
  importer. `cmEnforcedGroupOrders` reports `Severe implementation error in
  the Json to ElkGraph importer.`; `topdown.sizeApproximator` reports that its
  type must be registered with `ElkReflect`. Native `LayoutJSON` pins the same
  semantic errors exactly, without JavaScript stack frames.

## Staged gates

`upgrade-stages.json` is checked by `TestUpgradeManifestCoverage`; it covers
all eleven D2-selectable algorithms once and every curated D2 input once.

- `10-utility-providers`: Fixed, Box, seeded Random.
- `20-specialized-providers`: Stress, MR Tree, Radial, Force, both SPOrE
  providers, and Rectangle Packing.
- `30-layered-flat`: directions, branching, labels, cycles, self-loops, ports,
  and old model-order input migrated to the 0.12 profile.
- `31-layered-compound`: compounds, cross-hierarchy edges, and nested D2 cases.
- `32-layered-strategies`: D2-observed strategy-specific layer assignment.
- option metadata and the external complete D2 capture remain separate gates
  so their large mismatch lists do not hide algorithm progress.

Provider stages, curated fixtures, D2-profile random cases, and the complete D2
corpus all use `ExactTolerance`: even a repeatable one-ULP difference is
recorded. Codec and option-behavior differentials are exact as well.

## Completed compatibility gates

The completed port has the following strict results against the pinned
official 0.12.0 bundle:

- 11 of 11 provider smoke layouts are bit-exact.
- 15 of 15 curated fixtures are bit-exact.
- 322 of 322 captured pre-layout D2 graphs are bit-exact after applying the
  documented D2 0.12 profile migration.
- Deterministic D2-profile random layouts are bit-exact.
- The 51-added/14-removed metadata inventory is registered and classified;
  this metadata result is not a promise of full expert-option execution.
- Codec and option behavior exercised by the D2-facing gates compare exactly.

As a development diagnostic outside the contract, the broader generic-Layered
100-seed gate is structurally exact for every seed. Sixty-seven seeds are
bit-exact. The other 33 differ only at clipped edge start/end coordinates by
at most `1.13686837722e-13`; the diagnostic uses an explicit `2e-13` absolute
bound. The D2 contract does not use this tolerance.

## Historical pre-port baseline

For historical comparison only, the initial baseline was measured at native
commit `50aafa9` before the 0.12 implementation changes, using the corrected D2
profile:

- Exact common provider graph passes: Fixed, Box, Layered, Radial, SPOrE
  Overlap, Rectangle Packing.
- Exact common provider graph mismatches:
  - Random: one-ULP coordinate difference.
  - Stress: material coordinate difference (`0.000200013991616` at the first
    mismatch).
  - MR Tree: edge bend-point topology differs (one point versus two).
  - Force: material coordinate difference (`0.1155355867` at the first
    mismatch).
  - SPOrE Compaction: one-ULP coordinate difference.
- Source-shaped utility cases: Fixed and all four Box packing modes pass;
  priority-sensitive Box differs by 21 pixels and seeded Random differs by one
  ULP.
- Curated Layered corpus: 13 of 15 pass. The mismatches are
  `13_moon_issue8_isomorphic.json` (first difference 0.5 pixels) and
  `15_coffman_graham.json` (first difference 78 pixels).
- Metadata initially had all 51 new options missing, all 14 removed options
  still present, and an extra DisCo provider. These are not current results.

Run the D2-facing staged gate with:

```sh
ELKJS_ORACLE_ASSET=/path/to/elkjs-0.12.0/lib/elk.bundled.js \
ELK_GO_ORACLE_UPGRADE=1 \
go test ./internal/oracle -run CandidateUpgradeStages -count=1 -v
```
