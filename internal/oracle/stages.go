// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UpgradeManifest describes the ordered differential gates for the native
// 0.12 port. Earlier stages are intentionally smaller and less coupled so a
// mismatch identifies a provider or algorithm phase before the complete D2
// corpus is attempted.
type UpgradeManifest struct {
	OracleVersion     string         `json:"oracleVersion"`
	OracleCommit      string         `json:"oracleCommit"`
	OracleAssetSHA256 string         `json:"oracleAssetSHA256"`
	Stages            []UpgradeStage `json:"stages"`
}

type UpgradeStage struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Algorithms  []string `json:"algorithms,omitempty"`
	Fixtures    []string `json:"fixtures,omitempty"`
}

// LoadUpgradeManifest reads and validates the checked-in staged corpus.
func LoadUpgradeManifest() (UpgradeManifest, error) {
	path := filepath.Join(moduleRoot(), "testdata", "upgrade-stages.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return UpgradeManifest{}, fmt.Errorf("read upgrade manifest: %w", err)
	}
	var manifest UpgradeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return UpgradeManifest{}, fmt.Errorf("decode upgrade manifest: %w", err)
	}
	if manifest.OracleVersion != OracleVersion || manifest.OracleCommit != OracleCommit || manifest.OracleAssetSHA256 != OracleAssetSHA256 {
		return UpgradeManifest{}, fmt.Errorf("upgrade manifest oracle pin does not match runner constants")
	}
	seenStages := make(map[string]struct{}, len(manifest.Stages))
	for _, stage := range manifest.Stages {
		if stage.ID == "" {
			return UpgradeManifest{}, fmt.Errorf("upgrade manifest contains an empty stage ID")
		}
		if _, exists := seenStages[stage.ID]; exists {
			return UpgradeManifest{}, fmt.Errorf("upgrade manifest repeats stage %q", stage.ID)
		}
		seenStages[stage.ID] = struct{}{}
		for _, fixture := range stage.Fixtures {
			if filepath.IsAbs(fixture) || filepath.Clean(fixture) != fixture || filepath.Dir(fixture) == ".." {
				return UpgradeManifest{}, fmt.Errorf("stage %q has unsafe fixture path %q", stage.ID, fixture)
			}
			if _, err := os.Stat(filepath.Join(moduleRoot(), "testdata", fixture)); err != nil {
				return UpgradeManifest{}, fmt.Errorf("stage %q fixture %q: %w", stage.ID, fixture, err)
			}
		}
	}
	return manifest, nil
}

func providerMatrixInput(algorithm string) []byte {
	return []byte(fmt.Sprintf(`{
		"id":"root",
		"layoutOptions":{
			"elk.algorithm":%q,
			"elk.randomSeed":"7",
			"elk.direction":"DOWN",
			"elk.hierarchyHandling":"INCLUDE_CHILDREN",
			"elk.spacing.edgeNode":"40",
			"elk.layered.nodePlacement.bk.fixedAlignment":"BALANCED",
			"elk.layered.thoroughness":"8",
			"elk.layered.spacing.edgeEdgeBetweenLayers":"50",
			"elk.edgeLabels.inline":"false",
			"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
			"elk.layered.cycleBreaking.strategy":"GREEDY_MODEL_ORDER",
			"elk.nodeSize.constraints":"MINIMUM_SIZE",
			"elk.contentAlignment":"H_CENTER V_CENTER",
			"spacing.nodeNodeBetweenLayers":"70",
			"spacing.edgeNodeBetweenLayers":"40",
			"elk.spacing.nodeSelfLoop":"50"
		},
		"children":[
			{"id":"a","x":0,"y":0,"width":30,"height":20},
			{"id":"b","x":100,"y":10,"width":40,"height":30},
			{"id":"c","x":45,"y":100,"width":20,"height":25}
		],
		"edges":[
			{"id":"e1","sources":["a"],"targets":["b"]},
			{"id":"e2","sources":["b"],"targets":["c"]}
		]
	}`, algorithm))
}
