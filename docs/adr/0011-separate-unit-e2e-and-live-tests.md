---
status: accepted
date: 2026-08-15
superseded_by: ADR-0031 (generic `tests/live/` location only)
---

# Separate unit, end-to-end, and live tests

Ardents uses three independent test surfaces: unit tests beside their Go
Modules, cross-process end-to-end tests under `tests/e2e`, and real-container
network tests under `tests/live`. Test names describe behavior, not delivery
stages. Every test creates its own prerequisites and can run directly; no test
consumes an evidence receipt or verdict from another test run.

The ordinary developer loop runs deterministic unit tests without Docker or
wall-clock campaigns. End-to-end tests build and drive public commands as
separate processes. Live tests use the same public commands in real containers,
own their full setup and teardown, and are invoked explicitly where Docker is
available. A retained artifact may support debugging, but it is never a
prerequisite for another test.

This supersedes the staged qualification architecture and the requirement in
ADR-0009 that every change run every executable gate in one undifferentiated
command. ADR-0009 continues to select Go and the root-module layout. Security
or release claims may define explicit acceptance criteria, but they must be
ordinary independently runnable tests rather than a second evidence protocol.
