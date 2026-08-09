# Notices for elk-go

`elk-go` is produced and maintained by the elk-go contributors. It is a Go
port of behavior and source from the Eclipse Layout Kernel project and ELK.js.

## Upstream sources

- Eclipse Layout Kernel 0.8.1
  - https://github.com/eclipse-elk/elk
  - commit `5dc0f65c78778beb74a36e0f470c6fe6ecdcef67`
- ELK.js 0.8.2
  - https://github.com/kieler/elkjs
  - commit `cd0dc6b4e4bf1f221db04147f88b4d7c7041b834`
- GWT 2.9.0 Java runtime emulation
  - https://github.com/gwtproject/gwt
  - commit `8188fd9f8e1b70396ba02ce01b1d73a467225a64`
  - Copyright 2008 Google Inc.
  - Apache License 2.0; see `LICENSE.apache-2.0`
- MoonBit Community `moon_elk` test fixtures
  - https://github.com/moonbit-community/moon_elk
  - unreleased snapshot (no tag), commit
    `3761d04fcc42331a39cd2e879a939697327ed6c3`
  - Eclipse Public License 2.0; see `LICENSE`

## Trademarks

Eclipse Layout Kernel is a trademark of the Eclipse Foundation.

## Copyright

All upstream content is the property of its respective authors or their
employers. For more information regarding authorship, consult the source
repository histories and the copyright headers preserved in the ported files.

## Declared project license

The upstream ELK projects and this port are made available under the Eclipse
Public License 2.0. The two Go adaptations of GWT runtime routines identified
in `PROVENANCE.md` remain available under the Apache License 2.0. The source
code for this port is available at https://github.com/d2lang/elk-go.

SPDX-License-Identifier: EPL-2.0

## Upstream third-party content notice

Eclipse Layout Kernel 0.8.1 declares the following third-party content in its
project notice. Elk-go has no third-party Go module dependencies; this list is
preserved as part of the upstream notice for the source from which the port was
derived. Elk-go also adapts GWT 2.9.0's `java.util.PriorityQueue` and legacy
`java.util.Arrays` merge sort; their Google copyright and Apache 2.0 license
are preserved in the corresponding Go files and `LICENSE.apache-2.0`.

- ANTLR Runtime only 3.2: New BSD license
- Apache Commons Logging Jar 1.1.1: Apache License 2.0
- Apache Log4J 1.2.8: Apache License 2.0
- ASM 5.0.1: New BSD license
- Google Guava 15.0.0: Apache License 2.0
- Google Guice / Inject Core API 3.0.0: Apache License 2.0
- Ptolemy demo models: Ptolemy License

## Cryptography

Upstream content may contain encryption software. The country in which you are
currently located may have restrictions on the import, possession, use, or
re-export of encryption software. Before using any encryption software, check
the applicable laws, regulations, and policies.
