# ELK Radial provenance

This directory is a Go translation of Eclipse ELK commit
`5dc0f65c78778beb74a36e0f470c6fe6ecdcef67` (ELK 0.8.1). The Java sources are
licensed under EPL-2.0.

| Go file | ELK 0.8.1 source |
| --- | --- |
| `options.go` | `Radial.melk` and the Java enums in `options/`; the option registry values generated upstream from `Radial.melk` are represented directly in Go |
| `provider.go` | `Radial.melk`, `RadialLayoutProvider.java`, `RadialLayoutPhases.java`, `RadialUtil.java`; `p1position/EadesRadial.java`, `p1position/wedge/AnnulusWedgeByLeafs.java`, `AnnulusWedgeByNodeSpace.java`; `sorting/IDSorter.java`, `PolarCoordinateSorter.java`; `intermediate/CalculateGraphSize.java`; `intermediate/optimization/CrossingMinimizationPosition.java`, `EdgeLengthOptimization.java`, `EdgeLengthPositionOptimization.java`; `intermediate/overlaps/RadiusExtensionOverlapRemoval.java`; `intermediate/compaction/AbstractRadiusExtensionCompaction.java`, `AnnulusWedgeCompaction.java`, `GeneralCompactor.java`, `RadialCompaction.java`; and `p2routing/StraightLineEdgeRouter.java` |
| `provider_test.go` | Source-shaped tests derived from `EadesRadial.java` and `Radial.melk` |
| `provider_oracle_test.go` | Original differential fixtures targeting the frozen elkjs 0.8.2 bundle |

No code in this directory is copied from elkrs or a newer ELK release.
