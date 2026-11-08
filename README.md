# RIGG

## Table of Contents

- [Introduction](#introduction)
  - [Features](#features)
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
| [`x/`](x/) | X (Twitter) API client in Go, built on the [mygo](https://github.com/egoist/mygo) native template | v0.1.0 in progress — planned v1 scope: login, signup, timeline, and profile reading |
| `youtube/` | YouTube API client in Go | Planned |

The hardest upstream algorithms are treated as pure computation and will be re-derived independently; the upstream repository under `vendor/XApis/` is a protocol reference only.

### Attribution

This project uses the [MIT License](https://opensource.org/license/MIT) as its only license, recorded in [`/LICENSE`](LICENSE). Third-party components are excluded from this requirement.

The OG-Open-Source organization is regarded as an intangible entity rather than a legal person.

Except for third-party components, all content in this organization reserves only the right of attribution. Any use of this content, including use by artificial intelligence systems for training, analysis, or generation, **should** credit OG-Open-Source as the original author.

## Installation and Usage

### Prerequisites

- [Go](https://go.dev) 1.27.3 or later (matching the `go` directive in [`x/go.mod`](x/go.mod))
- Git with submodule support
- [mygo](https://github.com/egoist/mygo) — no separate installation required; it is fetched on demand as a Go tool dependency via `go run github.com/egoist/mygo/cmd/mygo@latest` or `go tool mygo` inside the project

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
| [mygo](https://github.com/egoist/mygo) | Go module dependency | Managed by the Go toolchain | See upstream | Desktop/web UI framework used by `x/` |

### Usage

Develop the `x` project with live reload:

```bash
cd x
go tool mygo dev
```

Build the `x` project as a web/Wasm static site:

```bash
cd x
go tool mygo build -platform web
```

Build the `x` project for the desktop and run the tests:

```bash
cd x
go tool mygo build
go test ./...
```

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
- Source code **should** pass the linter defined by the project before submission.
- Each project directory is one independent Go module; do not introduce a `go.work` file.
- Third-party code without a package manager lives under `/vendor/` as a submodule with its original license retained.
- Documentation **MUST** be written in Markdown.

### Versioning

Each project in this monorepo adopts [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 2.0.0 with `X.Y.Z` versions.

Every tag **MUST** match the pattern `[a-z0-9\.-]+`.

Tags are created when a version actually ships with its features; the current `x` version is 0.1.0 in progress and is not yet tagged.

### Issue Reporting

When reporting an issue, please include:

- The environment, including the operating system and the runtime version.
- The steps required to reproduce the problem.
- The expected behavior.
- The actual behavior.

## Acknowledgments

- [egoist/mygo](https://github.com/egoist/mygo) — the native Go UI framework scaffolding the `x` project
- [cv-cat/XApis](https://github.com/cv-cat/XApis) — protocol reference for the clean-room X rework
- [OG-Open-Source](https://github.com/OG-Open-Source) — the organization behind this monorepo

---

© 2025-2026 OG-Open-Source. Content in this organization reserves only the right of attribution.
