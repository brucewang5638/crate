# GEMINI Project Context

## Project Overview
- **Name**: Crate
- **Language**: Go
- **Purpose**: A build and packaging tool designed to manage component builds (JARs, binaries) and bundle them into RPM-based distributions/releases.
- **Key Components**:
    - `cmd/crate`: Main CLI entry point.
    - `pkg/builder`: Logic for building individual components.
    - `pkg/repo`: Logic for generating RPM repositories.
    - `pkg/config`: Configuration management.

## Current Architecture
- **CLI Commands**:
    - `-build`: Builds specific components or groups.
    - `-release`: Generates a final release tarball with a local RPM repo.
    - `-arch`, `-dist`: Cross-compilation/build configuration.
- **Workflow**:
    1.  Components defined in `config.yaml`.
    2.  `crate -build` creates RPMs (cached).
    3.  `crate -release` aggregates cached RPMs and creates a repo.

## Best Practices & Decisions
- **Environment Separation**: Adopting a separation between "Test/Build Environment" (dirty, functional testing) and "Packaging Environment" (clean, release generation).
- **Artifact Promotion**: Moving towards an explicit "promotion" step to transfer verified artifacts (JARs) from Test to Packaging.

## User Preferences
- **Language**: Chinese (always).
- **Style**: Explain principles deepLy before solutions.
