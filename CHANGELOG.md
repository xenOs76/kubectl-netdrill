## 0.2.2 (2026-07-22)

### CI

    Update Go version to 1.26.5

## 0.2.1 (2026-06-06)

### Feat

    Added --resolve-image flag to auto-resolve container :latest to semver-tagged releases
    Added MCP resource netdrill://container-tools with prompts for AWS creds, HTTPS/TLS checks, and network diagnostics

### Documentation

    Updated README with MCP configuration, image resolution behavior, and new MCP resources
    Added container-tools guide with bundled CLI utilities and usage examples

### Tests

    Added unit tests covering image resolution and MCP container-tools registration

### Chores / Dependencies

    Declared semver library as a direct dependency

### Refactor

    Minor internal label/env handling improvements

## 0.2.0 (2026-05-31)

### New Features

    Added MCP (Model Context Protocol) server support with kubectl netdrill mcp command for AI/LLM integration.
    Added pod command execution capability with stdout/stderr capture and exit code reporting.
    Added deployment management operations.
    Added bash completion support for new MCP command.

### Documentation

    Expanded README with comprehensive MCP usage guide, configuration examples, and security guardrails.

### Chores

    Updated dependencies to include MCP SDK support.
    Updated .gitignore to exclude vendor directory.

## 0.1.9 (2026-05-30)

### Chore

    Update Go dependencies

## 0.1.8 (2026-05-29)

### Fix

    Trigger new release for fix version number detection at build time

## 0.1.7 (2026-05-12)

    New Features

        Automatic AWS EKS token support for Pods and Deployments when AWS identity env vars are present, injecting a projected service-account token and mount so IAM roles can be assumed.

    Tests

        Added unit tests validating EKS token volume and mount wiring for Pods and Deployments.

    Chores

        Adjusted a dependency declaration to indirect.

## 0.1.6 (2026-04-30)

    Refactor

        move code to internal folder

## 0.1.5 (2026-04-24)

    New Features

        Added shell completion support for bash, zsh, fish and a CLI command to emit them.
        Added generated man page documentation for the CLI; manpages included in releases.

    Chores

        Release pipeline updated to build and package completions and manpages across targets.
        CI/workflow added for code checks and automated releases.
        Added dependency to support manpage generation.

## 0.1.4 (2026-04-23)

    New Features

        Added deployment subcommand to create persistent troubleshooting deployments with configurable replicas, resource limits, and labels.
        Added global flags (--service-account, --port, --env, --host-network) for shared configuration across commands.

    Documentation

        Updated README with deployment subcommand reference and detailed usage examples.

    Tests

        Added comprehensive tests for deployment creation functionality.

    Refactor

        Consolidated pod configuration flags to root command level.

## 0.1.3 (2026-04-15)

    New Features
        Added pod subcommand with support for custom ports, environment variables, and service accounts.

    Improvements
        Updated image pull policy to PullIfNotPresent for more efficient deployments.
        Clarified command descriptions for better user guidance.

    Documentation
        Removed AGENTS.md documentation file.

    Testing
        Added comprehensive unit test coverage across all command modules.

    Configuration
        Added SBOM generation for release artifacts.
        Updated project dependencies.

## 0.1.2 (2026-04-07)

    CI: add Krew pluging manifest generation to Goreleaser's config

## 0.1.1 (2026-04-07)

    New Features

        Added --node-selector flag to specify Kubernetes node labels for scheduling pods to specific nodes.

## 0.0.1 (2026-04-01)

### Feat

- initial import
