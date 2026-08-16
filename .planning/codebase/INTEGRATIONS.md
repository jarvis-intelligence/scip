# External Integrations

**Analysis Date:** 2026-08-16

## Overview

The `scip` library and CLI are offline tools: no network calls exist in the Go/TS/Rust/Haskell/JVM code paths (verified by grep across `cmd/scip/` and `bindings/go/scip/`). All external integrations live in CI/CD workflows (`.github/workflows/`) and package registries used for publishing the six bindings. The only data integration is an embedded, file-based SQLite writer.

## APIs & External Services

**Code intelligence (CI-only):**
- Sourcegraph.com - daily indexing of example repos and upload of SCIP indexes via Sourcegraph CLI (`src code-intel upload`)
  - Where: `.github/workflows/scip-examples.yaml` (cron `0 0 * * *`, skipped on forks via `github.repository == 'scip-code/scip'`)
  - SDK/Client: `src` binary downloaded from `https://sourcegraph.com/.api/src-cli/src_linux_amd64`; indexers run from Docker images `sourcegraph/{scip-java,scip-python,scip-typescript,scip-ruby,scip-clang,scip-dotnet}:latest`
  - Auth: env `SRC_ENDPOINT=https://sourcegraph.com/`, secret `SRC_ACCESS_TOKEN_DOTCOM_SCIP_SA`
  - Indexed targets: google/guava, apache/pekko, Textualize/rich, vuejs/core, Homebrew/brew, fmtlib/fmt, serilog/serilog

**Package registries (release-time):**
- crates.io - `scip` crate from `bindings/rust/` (`release.yaml` job `release-crate`, `cargo publish`)
  - Auth: secret `CRATES_TOKEN`
- Hackage - `scip` package from `bindings/haskell/` (job `publish-haskell-bindings`, `haskell-actions/hackage-publish@v1.1`)
  - Auth: secret `HACKAGE_TOKEN`
- Maven Central via Sonatype Central Portal - `org.scip-code:scip-java-bindings` and `scip-kotlin-bindings` (job `publish-jvm-bindings`)
  - SDK/Client: `central-publishing-maven-plugin` 0.11.0 (`release` profile in `bindings/java/pom.xml`, `bindings/kotlin/pom.xml`), GPG signing via `maven-gpg-plugin` 3.2.8
  - Auth: secrets `MAVEN_USERNAME`, `MAVEN_PASSWORD`, `MAVEN_GPG_PRIVATE_KEY` (workflow env `MAVEN_USERNAME`/`MAVEN_PASSWORD` map to `server-username`/`server-password` of `server-id: central` in `actions/setup-java@v5`)
- npm registry - `@scip-code/scip` from `bindings/typescript/` (job `publish-npm`, `JS-DevTools/npm-publish@v4`)
  - Auth: npm trusted publishing OIDC (`id-token: write` permission, Node 24 for npm >= 11.5.1); no static token
- GitHub Releases - CLI tarballs + sha256 checksums for linux/darwin amd64/arm64 (job `build-go-binaries`, `gh release create/upload/edit`)
  - Auth: `secrets.GITHUB_TOKEN`

**GitHub platform (CI):**
- GitHub App token - the `fix` job in `.github/workflows/ci.yaml` pushes module-tidy/vendor-hash corrective commits under an App identity (`actions/create-github-app-token@v3`)
  - Auth: vars `RENOVATE_FIX_APP_ID`, secret `RENOVATE_FIX_APP_PRIVATE_KEY`
- Automated review enforcement - `Automattic/action-required-review@v5` in `proto-review.yaml` requires CSC approvals (users `@jupblb`, `@CatherineGasnier`, `@jamydev`) on any PR touching `scip.proto` (uses `pull_request_target` + `github.token`)
- Renovate - dependency updates (`.github/renovate.json`); Nix manager enabled; `com.google.protobuf:*` and Kotlin versions managed manually

**Schema/protocol tooling:**
- Buf - protobuf lint/breaking/codegen config (`buf.yaml`, `buf.gen.yaml`); plugins run locally via Nix, no remote BSR push detected (purely local tooling despite the buf.build config format)
- protoc plugin ecosystem - `protoc-gen-go`, `protoc-gen-es`, `protoc-gen-rs` (protobuf-codegen 3.7.2), `proto-lens-protoc`, `protoc-gen-doc` (all pinned through `flake.nix`)

## Data Storage

**Databases:**
- SQLite (embedded, file-based only)
  - Where: `scip expt-convert` in `cmd/scip/convert.go` converts an index to a local `index.db` (default `--output index.db`)
  - Client: `zombiezen.com/go/sqlite` v1.4.2 (pure-Go port of modernc SQLite; no CGo, no server)
  - Connection: local file path CLI flag; occurrences stored as opaque blobs to control DB size

**File Storage:**
- Local filesystem only - the CLI reads `index.scip` (gzip/zstd-compressed protobuf) and writes snapshot/test files; no object storage integration

**Caching:**
- None at runtime. CI caches: GitHub Actions cache for Maven (`actions/cache@v6` in `scip-examples.yaml`, `cache: maven` in `jvm-bindings.yaml`), npm cache in `publish-npm`, and DeterminateSystems `magic-nix-cache-action@v14` for Nix

## Authentication & Identity

**Auth Provider:**
- Not applicable - the library/CLI implement no authentication and have no user-facing services. All credentials are CI/CD publishing secrets (see above); the Sourcegraph upload token is a CI service-account token confined to `.github/workflows/scip-examples.yaml`

## Monitoring & Observability

**Error Tracking:**
- None

**Logs:**
- Go standard library: `log` in `cmd/scip/main.go`, `log/slog` in `cmd/scip/convert.go`; no structured logging pipeline
- CPU profiling opt-in: `--cpu-profile` flag on `scip expt-convert` writes a pprof file (`runtime/pprof` in `cmd/scip/convert.go`); `bindings/go/scip/memtest/` contains memory-testing helpers for the library
- CI health gate: `ci-pass` job aggregates all matrix results in `.github/workflows/ci.yaml`

## CI/CD & Deployment

**Hosting:**
- None (library + CLI artifacts). Distribution channels: GitHub Releases (binaries), Go module proxy, npm, crates.io, Hackage, Maven Central

**CI Pipeline:**
- GitHub Actions, five workflows in `.github/workflows/`:
  - `ci.yaml` (pull_request) - `fix` job (same-repo PRs only) auto-tidies Go modules and re-computes Nix vendor hashes, then a Nix-eval'd matrix runs every `checks.*` and `packages.*` attribute from `flake.nix`/`checks.nix`
  - `jvm-bindings.yaml` (pull_request on `bindings/{java,kotlin}/**`) - Maven build; validates pom versions equal `cmd/scip/version.txt`; runs outside Nix because Kotlin resolves the Java artifact from a Maven repo
  - `proto-review.yaml` (pull_request_target on `scip.proto`) - enforce CSC review approvals
  - `release.yaml` (push to main changing `cmd/scip/version.txt`, or manual dispatch) - tag `vX.Y.Z` + `bindings/go/scip/vX.Y.Z`, draft release, publish all bindings, build 4-platform binaries, finalize
  - `scip-examples.yaml` (daily cron) - example-repo indexing + Sourcegraph upload
- Key third-party actions: `actions/checkout@v7`, `actions/setup-go@v7`, `actions/setup-java@v5` (temurin 11), `actions/setup-node@v7` (node 24), `DeterminateSystems/nix-installer-action@v22`, `dorny/paths-filter@v4`, `Mic92/nix-update` (via `nix run`)

## Environment Configuration

**Required env vars:**
- Runtime: none - the `scip` CLI is configured entirely through flags
- CI/release secrets: `SRC_ACCESS_TOKEN_DOTCOM_SCIP_SA`, `CRATES_TOKEN`, `HACKAGE_TOKEN`, `MAVEN_USERNAME`, `MAVEN_PASSWORD`, `MAVEN_GPG_PRIVATE_KEY`, `RENOVATE_FIX_APP_PRIVATE_KEY` (+ var `RENOVATE_FIX_APP_ID`); standard `GITHUB_TOKEN` for release/tag operations
- Build-time: `GOWORK=off` (module-isolated Go builds in Nix/CI), `CGO_ENABLED=0` (release binaries), `BUF_CACHE_DIR` override in the Nix formatting check

**Secrets location:**
- GitHub Actions secrets and variables only; no secrets or `.env` files exist in the repository (`.env*` absent; `config/secrets/` and similar paths absent)

## Webhooks & Callbacks

**Incoming:**
- None - no servers are hosted by this repo; `pull_request_target`/`pull_request_review` events in `proto-review.yaml` are the only externally-triggered flows (GitHub-hosted)

**Outgoing:**
- CI-time only: Sourcegraph index upload (`src code-intel upload -repo=... -file=index.scip`) and registry publishes (crates.io, Hackage, Maven Central, npm, GitHub Releases). No application-level outbound HTTP.

---

*Integration audit: 2026-08-16*
