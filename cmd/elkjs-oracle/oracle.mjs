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

// D2's patched elkjs 0.8.2 bundle and the pristine elkjs 0.8.2 bundle are
// behaviorally equivalent through the public asynchronous API used here.
const acceptedAssets = new Map([
  [
    'a297b92da4a257b95c129cc1d8cfb379e2c89bcd4442af90ac06fdd2b1e220b4',
    'D2 embedded elkjs 0.8.2 bundle',
  ],
  [
    'cd56bf0ddb7ad2587583461d523fdd974dc56b59efd20cdfee954e1112ff1a49',
    'upstream elkjs 0.8.2 bundled distribution',
  ],
]);

function usage(message) {
  if (message) {
    console.error(message);
  }
  console.error(
    'usage: node oracle.mjs --asset /path/to/elk.bundled.js [--known-layout-options]',
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
        'expected the pinned elkjs 0.8.2 bundle',
    );
  }

  const require = createRequire(import.meta.url);
  const loaded = require(asset);
  const ELK = loaded.default ?? loaded;
  if (typeof ELK !== 'function') {
    throw new TypeError(`${description} did not export the ELK constructor`);
  }

  const elk = new ELK();
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
