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
