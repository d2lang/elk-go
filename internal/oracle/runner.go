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
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	OracleVersion     = "0.12.0"
	OracleCommit      = "ff5771d7165445c42c408bb8a090c8035272218c"
	OracleAssetSHA256 = "1222e44f953ce7746af23801e723708f8e6f436b8b377a6a5fc7552f34a307b3"
)

// Runner invokes the pinned official elkjs 0.12.0 Node oracle.
type Runner struct {
	NodePath   string
	ScriptPath string
	AssetPath  string
}

// DefaultRunner discovers Node and this repository's runner script.
// ELKJS_ORACLE_NODE and ELKJS_ORACLE_ASSET select the executable and exact
// official JavaScript artifact. The asset is intentionally not discovered in
// a sibling checkout: that made it too easy to run against D2's former 0.8.2
// bundle while working on the 0.12 port.
func DefaultRunner() Runner {
	root := moduleRoot()
	node := os.Getenv("ELKJS_ORACLE_NODE")
	if node == "" {
		node, _ = exec.LookPath("node")
	}
	return Runner{
		NodePath:   node,
		ScriptPath: filepath.Join(root, "cmd", "elkjs-oracle", "oracle.mjs"),
		AssetPath:  os.Getenv("ELKJS_ORACLE_ASSET"),
	}
}

// Check explains why this runner cannot currently be used. Callers should use
// the returned text when skipping an oracle-dependent test.
func (runner Runner) Check() error {
	if runner.NodePath == "" {
		return fmt.Errorf("Node.js was not found; set ELKJS_ORACLE_NODE")
	}
	if _, err := os.Stat(runner.NodePath); err != nil {
		return fmt.Errorf("Node.js executable %q: %w", runner.NodePath, err)
	}
	if _, err := os.Stat(runner.ScriptPath); err != nil {
		return fmt.Errorf("oracle script %q: %w", runner.ScriptPath, err)
	}
	if runner.AssetPath == "" {
		return fmt.Errorf("ELKJS_ORACLE_ASSET is not set; point it at the official elkjs %s lib/elk.bundled.js", OracleVersion)
	}
	asset, err := os.ReadFile(runner.AssetPath)
	if err != nil {
		return fmt.Errorf("elkjs oracle asset %q: %w; set ELKJS_ORACLE_ASSET", runner.AssetPath, err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(asset))
	if digest != OracleAssetSHA256 {
		return fmt.Errorf("elkjs oracle asset %q has sha256 %s, want official elkjs %s sha256 %s", runner.AssetPath, digest, OracleVersion, OracleAssetSHA256)
	}
	return nil
}

// Layout sends one graph through the reference elkjs implementation.
func (runner Runner) Layout(ctx context.Context, input []byte) ([]byte, error) {
	if err := runner.Check(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, runner.NodePath, runner.ScriptPath, "--asset", runner.AssetPath)
	command.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run elkjs %s oracle: %s", OracleVersion, message)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}

// KnownLayoutAlgorithms returns the algorithm metadata exposed by the pinned
// elkjs constructor. Its ordered IDs are an observable compatibility surface:
// 0.12.0 contains eleven providers and no longer contains DisCo.
func (runner Runner) KnownLayoutAlgorithms(ctx context.Context) ([]byte, error) {
	if err := runner.Check(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, runner.NodePath, runner.ScriptPath,
		"--asset", runner.AssetPath, "--known-layout-algorithms")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("read elkjs %s algorithm metadata: %s", OracleVersion, message)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}

// KnownLayoutOptions returns the metadata surface exposed by the pinned
// elkjs constructor. It is used to keep option identity and suffix resolution
// compatible even for options that are not exercised by D2's default profile.
func (runner Runner) KnownLayoutOptions(ctx context.Context) ([]byte, error) {
	if err := runner.Check(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, runner.NodePath, runner.ScriptPath,
		"--asset", runner.AssetPath, "--known-layout-options")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("read elkjs %s option metadata: %s", OracleVersion, message)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}

// LayoutFunc is the byte-level JSON layout seam used by differential tests.
type LayoutFunc func([]byte) ([]byte, error)

// Differential runs input through the reference and candidate layouts, then
// applies the strict structural comparator.
func Differential(ctx context.Context, runner Runner, input []byte, candidate LayoutFunc, tolerance Tolerance) error {
	want, err := runner.Layout(ctx, input)
	if err != nil {
		return err
	}
	got, err := callCandidate(candidate, input)
	if err != nil {
		return fmt.Errorf("candidate layout: %w", err)
	}
	return CompareJSON(want, got, tolerance)
}

func callCandidate(candidate LayoutFunc, input []byte) (output []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			output = nil
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return candidate(input)
}

func moduleRoot() string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}
