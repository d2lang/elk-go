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
	d2AssetSHA256       = "a297b92da4a257b95c129cc1d8cfb379e2c89bcd4442af90ac06fdd2b1e220b4"
	upstreamAssetSHA256 = "cd56bf0ddb7ad2587583461d523fdd974dc56b59efd20cdfee954e1112ff1a49"
)

// Runner invokes the pinned elkjs 0.8.2 Node oracle.
type Runner struct {
	NodePath   string
	ScriptPath string
	AssetPath  string
}

// DefaultRunner discovers Node, this repository's runner script, and D2's
// sibling checkout. ELKJS_ORACLE_NODE and ELKJS_ORACLE_ASSET override Node and
// the JavaScript asset respectively.
func DefaultRunner() Runner {
	root := moduleRoot()
	node := os.Getenv("ELKJS_ORACLE_NODE")
	if node == "" {
		node, _ = exec.LookPath("node")
	}
	asset := os.Getenv("ELKJS_ORACLE_ASSET")
	if asset == "" {
		asset = filepath.Join(root, "..", "d2", "d2layouts", "d2elklayout", "elk.js")
	}
	return Runner{
		NodePath:   node,
		ScriptPath: filepath.Join(root, "cmd", "elkjs-oracle", "oracle.mjs"),
		AssetPath:  asset,
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
	asset, err := os.ReadFile(runner.AssetPath)
	if err != nil {
		return fmt.Errorf("elkjs oracle asset %q: %w; set ELKJS_ORACLE_ASSET", runner.AssetPath, err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(asset))
	if digest != d2AssetSHA256 && digest != upstreamAssetSHA256 {
		return fmt.Errorf("elkjs oracle asset %q has unrecognized sha256 %s", runner.AssetPath, digest)
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
		return nil, fmt.Errorf("run elkjs 0.8.2 oracle: %s", message)
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
		return nil, fmt.Errorf("read elkjs 0.8.2 option metadata: %s", message)
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
