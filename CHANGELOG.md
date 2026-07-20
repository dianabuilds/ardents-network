# Changelog

All notable changes are recorded here. The project follows semantic versioning
after the first accepted `v1.0.0` release; pre-release builds use explicit
version identifiers and do not imply compatibility.

## [Unreleased]

### Added

- canonical Waku-backed node runtime and supported transport profiles;
- capability-bound private discovery and data exchange;
- local operator API, CLI/TUI, configuration, authorization, audit, diagnostics,
  production observability, and self-forming Docker deployment;
- workload/hosted-service control and encrypted multi-node data availability;
- build identity and release artifact contract.

### Security

- loopback-only control and observability listeners;
- private retained state, fail-closed continuity, bounded network/resource
  controls, explicit dependency exceptions, and redacted diagnostics.

### Known limitations

- no accepted production release or compatibility promise yet;
- private realm capability provisioning remains deployment-owned;
- final CI and release acceptance gates remain pending.
