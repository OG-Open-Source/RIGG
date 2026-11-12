# RIGG

## Table of Contents

- [Introduction](#introduction)
  - [Features](#features)
  - [X Engine](#x-engine)
  - [Attribution](#attribution)
- [Installation and Usage](#installation-and-usage)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Third-Party Components](#third-party-components)
  - [Usage](#usage)
- [Contributing](#contributing)
  - [Code of Conduct](#code-of-conduct)
  - [Development Guidelines](#development-guidelines)
  - [Versioning](#versioning)
  - [Issue Reporting](#issue-reporting)
- [Acknowledgments](#acknowledgments)

## Introduction

RIGG is a monorepo that collects clean-room rewrites of third-party API services in Go. The first target is X (formerly Twitter); YouTube is planned. Upstream services are studied for protocol behavior only, and all code in this repository is written from scratch — no upstream source code is copied into any published artifact.

### Features

Current and planned projects, one independent Go module per directory:

| Project | Description | Status |
| --- | --- | --- |
| [`X/`](X/) | X (Twitter) web client engine in Go plus its MyGO desktop app, module `github.com/OG-Open-Source/RIGG/X` | v1 delivered, L2-accepted — cookie login, home timeline reading with cursor paging |
| `youtube/` | YouTube API client in Go | Planned |

The hardest upstream algorithms are treated as pure computation and will be re-derived independently; the upstream repository under `vendor/XApis/` is a protocol reference only.

### X Engine

The first delivered project is the **X engine** ([`X/`](X/), Go module `github.com/OG-Open-Source/RIGG/X`): a clean-room reimplementation in Go 1.27.3 of the X (Twitter) web client engine, built against [`XAPIS_BEHAVIOR_SPEC.md`](XAPIS_BEHAVIOR_SPEC.md) — the frozen behavioral contract of the upstream XApis client — and embedded as the native engine of a [MyGO](https://github.com/egoist/mygo) desktop app. The user pastes their own X cookie and immediately browses the home timeline in a native desktop UI, with continuous cursor paging.

- **Saving RAM**: a single pure-Go (zero-cgo) binary replaces high-footprint SPA/WebView or Python resident stacks — resident memory is measured in megabytes instead of dragging an interpreter, a browser engine, and a database consuming hundreds. Live measurement: ~30 MiB peak (11.3 MiB idle → 26.0 MiB with the timeline loaded).
- **Native UI, zero serialization boundary**: the app uses the MyGO native UI template (`package ui`, no WebView, no HTML/JS); the UI calls the engine in the same process.
- **Anti-bot wall**: X identifies non-browser traffic by TLS fingerprint and header characteristics; the transport layer uses `bogdanfinn/tls-client` (uTLS Chrome TLS fingerprint emulation, pure Go, zero cgo).
- **Comparable behavioral contract**: the spec freezes behavior requirement by requirement (`REQ-*`); the implementation uses those requirement numbers as its work units. The engine lives in the `X/internal/xapi` package tree and **MUST NOT** import any MyGO package (CI-enforced by `importban_test.go`), preserving byte-level comparability against the spec and future mechanical extraction.
- **Mechanically classifiable errors**: errors cross boundaries with a typed classification (stable `kind`/`code`), so the app layer can mechanically distinguish unavailable content, invalid credentials, rate-limit backoff, and transport failure.
- **Engine layout**: `X/internal/xapi/{fingerprint,transport,auth,headers,graphql,api}` plus `xerrors` for typed wire-error kinds; the GraphQL header family ships in v1 with the six-family ordered optional-slot model.
- **v1 scope**: cookie-import login plus home timeline reading with cursor paging. Writes, media, articles, direct messages, password login, and the web console are out of scope for v1.

Live L2 acceptance: Viewer identity validation 3/3; HomeTimeline 28 posts on the first page; paged to the 10-page cap (298 cumulative posts, zero cross-page duplicates); rate-limit/flood codes 226/344 = 0. All spec interpretations and deviations are logged in [`docs/DECISIONS.md`](docs/DECISIONS.md); the XCTID header is deferred per the P0 probe verdict and kept as an optional slot.

### Attribution

This project uses the [MIT License](https://opensource.org/license/MIT) as its only license, recorded in [`/LICENSE`](LICENSE). Third-party components are excluded from this requirement.

The OG-Open-Source organization is regarded as an intangible entity rather than a legal person.

Except for third-party components, all content in this organization reserves only the right of attribution. Any use of this content, including use by artificial intelligence systems for training, analysis, or generation, **should** credit OG-Open-Source as the original author.

For the X engine specifically:

- 本專案的行為規格（`XAPIS_BEHAVIOR_SPEC.md`）由 **OG-Open-Source** 整理並凍結——OG-Open-Source 為規格整理者。
- 上游**來源專案**為 [XApis](https://github.com/cv-cat/XApis)（作者 **cv-cat**）；本專案為依規格進行的 clean-room 重寫，不繼承原專案程式碼。
- **授權狀態註記**（規格 §19.1-3）：上游 XApis 的授權狀態尚未確認；確認前本 repo 維持**私有**、不對外散布。發布（轉為公開）前**必須**完成授權確認。

## Installation and Usage

### Prerequisites

- [Go](https://go.dev) 1.27.3 or later (matching the `go` directive in [`X/go.mod`](X/go.mod))
- Git with submodule support
- [Bun](https://bun.sh) — latest stable, required by the MyGO toolchain
- [mygo](https://github.com/egoist/mygo) v0.9.0 — no separate installation required; it is fetched on demand as a Go tool dependency via `go run github.com/egoist/mygo/cmd/mygo@latest` or `go tool mygo` inside the project

### Installation

Clone the repository and initialize its submodules:

```bash
git clone --recurse-submodules https://github.com/RIGG/RIGG.git
cd RIGG
```

If the repository was already cloned without submodules:

```bash
git submodule update --init --recursive
```

Verify that all submodules are initialized:

```bash
git submodule status
```

### Third-Party Components

Go dependencies are resolved through the Go module proxy with checksums committed in each project's `go.sum` file.

| Component | Source | Location | License | Notes |
| --- | --- | --- | --- | --- |
| `.github` | [OG-Open-Source/.github](https://github.com/OG-Open-Source/.github) | `.github/` (submodule) | MIT (retained) | Organization-wide documentation standard |
| XApis | [cv-cat/XApis](https://github.com/cv-cat/XApis) | `vendor/XApis/` (submodule) | **No license file** | Used strictly as an internal protocol reference for clean-room development. It is not copied into any published artifact and is not a runtime dependency. Its Python implementation is never executed by this project. |
| [mygo](https://github.com/egoist/mygo) | Go module dependency | Managed by the Go toolchain | See upstream | Desktop/web UI framework used by the X app in `X/` |
| [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) | Go module dependency | Managed by the Go toolchain | See upstream | TLS-fingerprint transport used by the X engine (`X/internal/xapi/transport`); see [`docs/DECISIONS.md`](docs/DECISIONS.md) for the capability comparison and pinning evidence |

### Usage

Develop the X app with live reload:

```bash
cd X
go tool mygo dev
```

Build the X app as a web/Wasm static site:

```bash
cd X
go tool mygo build -platform web
```

Build the X app for the desktop and run the tests:

```bash
cd X
go tool mygo build
go test -race ./...
```

`go test -race ./...` must pass for every package.

## Contributing

### Code of Conduct

All participants **MUST** follow the [Code of Conduct](CODE_OF_CONDUCT.md) in `/CODE_OF_CONDUCT.md`, which adopts the [Contributor Covenant](https://www.contributor-covenant.org) version 3.0.

To contribute to a repository:

1. Fork the repository.
2. Create a feature branch.
3. Commit your changes following the commit message convention.
4. Open a pull request.

### Development Guidelines

- Source code and comments are written in English.
- Files **MUST** be saved in UTF-8 encoding with LF line endings, enforced by [`.gitattributes`](.gitattributes).
- Commit messages **MUST** follow the [Conventional Commits](https://www.conventionalcommits.org) specification, version 1.0.0.
- Source code **should** pass the linter defined by the project before submission (`go vet` for `X/`).
- Each project directory is one independent Go module; do not introduce a `go.work` file.
- Third-party code without a package manager lives under `/vendor/` as a submodule with its original license retained.
- The `X/internal/xapi` package tree **MUST NOT** import any MyGO package (CI-enforced static check).
- Interpretations of, and deviations from, the X behavioral spec **MUST** be recorded in [`docs/DECISIONS.md`](docs/DECISIONS.md) without modifying the frozen spec.
- Documentation **MUST** be written in Markdown.

### Versioning

Each project in this monorepo adopts [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 2.0.0 with `X.Y.Z` versions.

Every tag **MUST** match the pattern `[a-z0-9\.-]+`.

Tags are created when a version actually ships with its features; the current `X` version is 0.1.0 (engine v1, L2-accepted) and is not yet tagged.

### Issue Reporting

When reporting an issue, please include:

- The environment, including the operating system and the runtime version.
- The steps required to reproduce the problem.
- The expected behavior.
- The actual behavior.

## Acknowledgments

- [egoist/mygo](https://github.com/egoist/mygo) — the native Go UI framework scaffolding the X project
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) — the TLS-fingerprint transport behind the X engine
- [cv-cat/XApis](https://github.com/cv-cat/XApis) — protocol reference for the clean-room X rework
- [OG-Open-Source](https://github.com/OG-Open-Source) — the organization behind this monorepo

---

© 2025-2026 OG-Open-Source. Content in this organization reserves only the right of attribution.
