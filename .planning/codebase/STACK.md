# Technology Stack

**Analysis Date:** 2026-08-16

## Overview

SCIP ("skip") is a language-agnostic Protobuf protocol for code intelligence indexing. The repo is a polyglot monorepo: a Go CLI (`cmd/scip`), a rich Go library (`bindings/go/scip`), a tree-sitter test indexer (`reprolang/`), and generated bindings for Rust, TypeScript, Haskell, Java, and Kotlin — all produced from the single schema `scip.proto` via Buf. Nix is the single-entry dev/build toolchain. Current version: 0.9.0 (`cmd/scip/version.txt`).

## Languages

**Primary:**
- Go 1.25.0 - Three-module Go workspace (`go.work`): root module `github.com/scip-code/scip` (the CLI in `cmd/scip/`), library module `github.com/scip-code/scip/bindings/go/scip` (hand-written helpers + generated `bindings/go/scip/scip.pb.go`), and test-indexer module `github.com/scip-code/scip/reprolang`
- Protocol Buffers (proto3) - `scip.proto` is the product itself: the SCIP schema (~35KB, package `scip`)

**Secondary:**
- Rust 2021 edition (MSRV 1.81.0) - `bindings/rust/Cargo.toml`; hand-written wrappers over generated `bindings/rust/src/generated/`
- TypeScript (ESM) - `bindings/typescript/` (`@scip-code/scip` npm package); generated `scip_pb.ts` compiled with `tsc`
- Haskell (GHC2021; tested GHC 9.4.8–9.10.3) - `bindings/haskell/scip.cabal`; proto-lens generated
- Java 11 - `bindings/java/pom.xml` (`org.scip-code:scip-java-bindings`); protoc-built-in generated code
- Kotlin 2.0.21 (pinned `<2.1.0` by Renovate rule) - `bindings/kotlin/pom.xml` (`org.scip-code:scip-kotlin-bindings`)
- Tree-sitter grammar DSL (JavaScript) - `reprolang/grammar.js`, generated C parser checked in at `reprolang/grammar/parser.c` (ABI 14)
- Nix - `flake.nix`, `checks.nix` (declarative CI/check matrix)
- Bash - `reprolang/generate-tree-sitter-parser.sh`
- Yaml/Json - GitHub workflows, Renovate config

## Runtime

**Environment:**
- Go toolchain 1.25.0 (workspace `go.work` spans `.` + `bindings/go/scip` + `reprolang`; per-module builds set `GOWORK=off` in Nix/CI)
- Node.js - required only for tree-sitter parser generation and TypeScript binding build (`bindings/typescript/package.json`)
- Nix (nixos-26.05 channel, `flake.lock`) - canonical entry point: `nix run .#proto-generate`, `nix develop` provides go, cargo, rustc, nodejs, tree-sitter, cabal, ghc
- CLI binaries are static (`CGO_ENABLED=0` in `release.yaml`) — no libc dependency; SQLite is pure-Go so CGo stays off

**Package Manager:**
- Go modules - `go.mod`/`go.sum` at root, `bindings/go/scip/go.mod`, `reprolang/go.mod`; lockfiles present
- Cargo - `bindings/rust/Cargo.lock` present
- npm - `bindings/typescript/package-lock.json` present
- Cabal - `bindings/haskell/scip.cabal` (no lockfile; built via `callCabal2nix` in `checks.nix`)
- Maven - `bindings/java/pom.xml`, `bindings/kotlin/pom.xml` (built outside Nix because Kotlin resolves Java from Maven Central; see `.github/workflows/jvm-bindings.yaml`)
- Nix flake - `flake.lock` present

## Frameworks

**Core:**
- `github.com/urfave/cli/v3` v3.10.1 - CLI framework for the `scip` binary (`cmd/scip/main.go`); subcommands: `lint`, `print`, `snapshot`, `stats`, `test`, `expt-convert`
- `google.golang.org/protobuf` v1.36.12 - Protobuf runtime for Go library and CLI
- Buf - protobuf codegen orchestrator (`buf.yaml` v1 lint/breaking config; `buf.gen.yaml` v2 plugin list)

**Testing:**
- `github.com/stretchr/testify` v1.11.1 - assertions across all three Go modules
- `github.com/hexops/autogold/v2` v2.3.1 - golden snapshot testing in `reprolang/` and `cmd/scip` (`go test ./cmd/scip -update-snapshots`)
- `pgregory.net/rapid` v1.3.0 - property-based testing in `bindings/go/scip` (e.g. `symbol_parser.go` roundtrips)
- `github.com/google/go-fuzz` + `gofuzz` - fuzzing support in `bindings/go/scip`
- `pretty_assertions` 1.4.1 - Rust dev-dependency (`bindings/rust/Cargo.toml`)
- Nix `checks` - every binding builds in CI as a flake check (`checks.nix`), including a `reprolang-generated` check that regenerates the tree-sitter parser and diffs it

**Build/Dev:**
- Nix flake (`flake.nix`) - packages: `scip` (Go CLI), `proto-generate` (one-command protobuf regen for ALL bindings), `default`; checks: `github-actions` (action-validator), `formatting` (prettier/buf/gofmt/goimports/nixfmt), `go-bindings`, `haskell-bindings`, `reprolang`, `reprolang-generated`, `rust-bindings`, `typescript-bindings`
- protoc plugin set (pinned in `flake.nix`): `protoc-gen-go`, `protoc-gen-es` (TypeScript), `protoc-gen-rs` (= `protobuf-codegen` crate 3.7.2, built from source in Nix), `proto-lens-protoc` (Haskell), protoc-builtin `java`/`kotlin`, `protoc-gen-doc` (generates `docs/scip.md` via `docs/scip.sprig` template)
- Prettier - repo-wide formatter for ts/js/json/md/yml (`.prettierrc`, `.prettierignore`)
- tree-sitter CLI - `tree-sitter generate --abi 14` for reprolang parser
- Maven - JVM bindings build/publish
- Renovate - dependency automation (`.github/renovate.json`); protobuf-java and Kotlin versions are deliberately managed by hand (packageRules disable/pin them)

## Key Dependencies

**Critical (root module `go.mod`):**
- `github.com/urfave/cli/v3` v3.10.1 - CLI surface
- `google.golang.org/protobuf` v1.36.12 - index (de)serialization; note `bindings/go/scip/parse.go` also contains a hand-written fast protobuf varint parser for performance
- `zombiezen.com/go/sqlite` v1.4.2 (modernc pure-Go SQLite) - `scip expt-convert` writes `index.db` (`cmd/scip/convert.go`)
- `github.com/klauspost/compress` v1.19.2 - zstd decompression of indexes (`cmd/scip/print.go`, `convert.go`)
- `github.com/hhatto/gocloc` v0.7.0 - comment/line counting for `scip stats` (`cmd/scip/stats.go`)
- `github.com/montanaflynn/stats` v0.12.3 - percentile/mean/stddev aggregation in `scip stats`
- `github.com/k0kubun/pp/v3` v3.5.2 - pretty-printing index output

**Library module (`bindings/go/scip/go.mod`):**
- `github.com/sourcegraph/beaut` - symbol formatting used by snapshot output
- `github.com/hexops/gotextdiff` - diffing for snapshot tests
- `github.com/fatih/color` v1.19.0 - colored output (respects https://no-color.org/, see `cmd/scip/print.go`)

**reprolang module (`reprolang/go.mod`):**
- `github.com/tree-sitter/go-tree-sitter` v0.25.0 - CGo bindings to the generated parser (`reprolang/grammar/binding.go`); the only CGo dependency in the repo and it is test-only

**Rust binding (`bindings/rust/Cargo.toml`):**
- `protobuf` =3.7.2 (exact pin — must match the `protoc-gen-rs` codegen version in `flake.nix`)

**TypeScript binding (`bindings/typescript/package.json`):**
- `@bufbuild/protobuf` ^2.11.0 - runtime for protoc-gen-es output

**JVM bindings (`bindings/{java,kotlin}/pom.xml`):**
- `com.google.protobuf:protobuf-java`/`protobuf-kotlin` 4.34.2 (pinned to match Nix protoc; Renovate updates disabled)

## Configuration

**Environment:**
- No runtime config files or env vars for the CLI — all configuration is CLI flags (`--from`, `--project-root`, `--comment-syntax`, `--output`, `--cpu-profile`; defaults in `cmd/scip/main.go`)
- Version single-source-of-truth: `cmd/scip/version.txt` (0.9.0); every binding manifest version must match it and is asserted in CI (`checks.nix` assertions, `jvm-bindings.yaml` xpath check)
- No `.env` files exist in the repo

**Build:**
- `buf.yaml` (lint DEFAULT minus naming exceptions; breaking check `FILE`), `buf.gen.yaml` (six codegen plugins)
- `flake.nix` / `checks.nix` / `flake.lock` - Nix build + check matrix
- `go.work` - Go multi-module workspace
- `.prettierrc` / `.prettierignore` - Prettier (semi: false, singleQuote, es5 trailing commas)
- `.gitattributes`, `.gitignore` - repo hygiene (ignores `vendor/`, `/bindings/typescript/dist/`, `/result`)

## Platform Requirements

**Development:**
- Nix is the only required dependency (`docs/Development.md`); `nix develop` supplies Go, Rust, Node, GHC/cabal, tree-sitter
- Without Nix: Go 1.25+, plus per-binding toolchains only when touching that binding
- macOS (arm64/amd64) or Linux (amd64/arm64) hosts; CI runs `ubuntu-latest` and `macos-latest`

**Production:**
- The `scip` CLI ships as static tarballs for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` via GitHub Releases (`release.yaml`)
- Published artifacts: Go module, npm `@scip-code/scip`, crates.io `scip`, Hackage `scip`, Maven Central `org.scip-code:{scip-java-bindings,scip-kotlin-bindings}`

---

*Stack analysis: 2026-08-16*
