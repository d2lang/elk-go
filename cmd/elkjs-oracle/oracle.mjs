// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';

// This is the unmodified bundled distribution from the official elkjs 0.12.0
// tag. Refuse every other artifact so an in-progress native port cannot
// accidentally compare itself with an older checkout or a locally rebuilt
// bundle.
const acceptedAssets = new Map([
  [
    '1222e44f953ce7746af23801e723708f8e6f436b8b377a6a5fc7552f34a307b3',
    'official elkjs 0.12.0 bundled distribution',
  ],
]);

function usage(message) {
  if (message) {
    console.error(message);
  }
  console.error(
    'usage: node oracle.mjs --asset /path/to/elk.bundled.js ' +
      '[--known-layout-algorithms | --known-layout-options]',
  );
  process.exit(2);
}

function assetArgument(argv) {
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--asset' && i + 1 < argv.length) {
      return resolve(argv[i + 1]);
    }
  }
  usage('missing required --asset argument');
}

// `$H` is an internal GWT identity hash. It is not part of elkjs's typed JSON
// API and D2 discards it when unmarshalling the result into Go structs.
function withoutGWTIdentityHashes(value) {
  if (Array.isArray(value)) {
    return value.map(withoutGWTIdentityHashes);
  }
  if (value !== null && typeof value === 'object') {
    const result = {};
    for (const [key, child] of Object.entries(value)) {
      if (key !== '$H') {
        result[key] = withoutGWTIdentityHashes(child);
      }
    }
    return result;
  }
  return value;
}

async function main() {
  const argv = process.argv.slice(2);
  const asset = assetArgument(argv);
  const source = await readFile(asset);
  const digest = createHash('sha256').update(source).digest('hex');
  const description = acceptedAssets.get(digest);
  if (!description) {
    throw new Error(
      `unrecognized ELK oracle asset ${asset} (sha256 ${digest}); ` +
        'expected the pinned official elkjs 0.12.0 bundle',
    );
  }

  const require = createRequire(import.meta.url);
  const loaded = require(asset);
  const ELK = loaded.default ?? loaded;
  if (typeof ELK !== 'function') {
    throw new TypeError(`${description} did not export the ELK constructor`);
  }

  const elk = new ELK();
  if (argv.includes('--known-layout-algorithms')) {
    const algorithms = await elk.knownLayoutAlgorithms();
    process.stdout.write(
      `${JSON.stringify(withoutGWTIdentityHashes(algorithms))}\n`,
    );
    return;
  }
  if (argv.includes('--known-layout-options')) {
    const options = await elk.knownLayoutOptions();
    process.stdout.write(`${JSON.stringify(withoutGWTIdentityHashes(options))}\n`);
    return;
  }

  const input = await new Promise((resolveInput, rejectInput) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => {
      data += chunk;
    });
    process.stdin.on('end', () => resolveInput(data));
    process.stdin.on('error', rejectInput);
  });
  if (input.trim() === '') {
    usage('stdin must contain one ELK JSON graph');
  }

  const graph = JSON.parse(input);
  const result = await elk.layout(graph);
  process.stdout.write(`${JSON.stringify(withoutGWTIdentityHashes(result))}\n`);
}

main().catch((error) => {
  console.error(error?.stack ?? String(error));
  process.exit(1);
});
