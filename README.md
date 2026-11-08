# OG-Open-Source README Standard

This document defines the standard README structure and the management rules for every repository within the OG-Open-Source organization.

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

OG-Open-Source develops and shares innovative open-source software. Its mission is to build and sustain high-quality, community-driven projects that empower developers and users worldwide.

### Features

Each repository **should** describe its highlights and the problems it solves. Repositories can adapt this section to their needs, using the following structure:

- The core capabilities of the project.
- The specific problem the project solves.
- The principal advantages over comparable approaches.

### Attribution

Every project in this organization **MUST** use the [MIT License](https://opensource.org/license/MIT) as its only license, recorded in `/LICENSE`. Third-party components are excluded from this requirement.

The OG-Open-Source organization is regarded as an intangible entity rather than a legal person.

Except for third-party components, all content in this organization reserves only the right of attribution. Any use of this content, including use by artificial intelligence systems for training, analysis, or generation, **should** credit OG-Open-Source as the original author.

## Installation and Usage

### Prerequisites

List every environment requirement, including the runtime and its minimum version, such as Node.js 20.0 or later or Python 3.12 or later. Each repository **should** state the exact version required for each dependency.

### Installation

Clone the repository and initialize its submodules:

```bash
git clone https://github.com/OG-Open-Source/REPOSITORY_NAME.git
cd REPOSITORY_NAME
git submodule update --init --recursive
```

Every repository **MUST** include the organization README standard through the `.github` repository as a submodule at `/.github/`:

```bash
git submodule add https://github.com/OG-Open-Source/.github.git .github
```

### Third-Party Components

Third-party libraries **should** be installed through the package manager of their language ecosystem whenever that package manager provides them, and the package manager's lock file **MUST** be committed. Packages installed through a package manager are not stored in `/vendor/`.

When no package manager provides a library, the library **MUST** be introduced with the `git submodule add` command and moved into the `/vendor/` directory with the `git mv` command:

```bash
git submodule add https://github.com/THIRD_PARTY_OWNER/THIRD_PARTY_REPOSITORY.git THIRD_PARTY_REPOSITORY
git mv THIRD_PARTY_REPOSITORY vendor/THIRD_PARTY_REPOSITORY
git commit -m "chore: vendor THIRD_PARTY_REPOSITORY"
```

Each third-party component **MUST** retain its original license file without modification.

### Usage

Provide at least one example for each primary function, showing the command or code and its expected output:

```bash
REPOSITORY_NAME --option VALUE
```

## Contributing

Repositories that accept pull requests **MUST** link the Code of Conduct. Issues are optional.

### Code of Conduct

All participants **MUST** follow the [Code of Conduct](CODE_OF_CONDUCT.md) in `/CODE_OF_CONDUCT.md`, which adopts the [Contributor Covenant](https://www.contributor-covenant.org) version 3.0.

To contribute to a repository:

1. Fork the repository.
2. Create a feature branch.
3. Commit your changes following the commit message convention.
4. Open a pull request.

### Development Guidelines

- Except for third-party components, repositories **MUST** adopt consensus standards that are free of charge and applicable to code. Where a [Request for Comments](https://www.rfc-editor.org/) document applies, it takes priority over other standards.
- Commit messages **can** follow the [Conventional Commits](https://www.conventionalcommits.org) specification, version 1.0.0.
- Changes **can** be recorded in `/CHANGELOG.md` following the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) convention, version 1.1.0.
- Source code **should** pass the linter defined by the project before submission.
- Documentation **MUST** be written in Markdown.
- Files **MUST** be saved in UTF-8 encoding.
- Special files, meaning files that Git hosting platforms recognize by community consensus, **MUST** have names matching `^[A-Z_]+\.md$`, such as `/README.md` and `/CODE_OF_CONDUCT.md`. `/LICENSE` is the sole exception. Ordinary source code is not subject to this rule.

### Versioning

Each project **MUST** adopt exactly one of the following versioning schemes:

- [Semantic Versioning](https://semver.org/spec/v2.0.0.html) 2.0.0.
- UNIX timestamp.
- Date in the `YYYY.MM.DD` format.

Every tag **MUST** match the pattern `[a-z0-9\.-]+`.

### Issue Reporting

Repositories that accept issues **should** provide an issue template that requests the following information:

- The environment, including the operating system and the runtime version.
- The steps required to reproduce the problem.
- The expected behavior.
- The actual behavior.

## Acknowledgments

We thank every contributor who has helped improve this project. View the full list of contributors on the [contributors graph](https://github.com/OG-Open-Source/.github/graphs/contributors).

---

© 2025-2026 OG-Open-Source. Content in this organization reserves only the right of attribution.
