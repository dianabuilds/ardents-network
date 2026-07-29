# MR-05 security audit — run 1

Target commit: `3e4b6c6`

Scope: MR-05 logical implementation range `4512baf..3e4b6c6`, concentrated on
private-LAN manifest reconciliation, protected proof admission, endpoint
publication/withdrawal, time/replay behavior, configuration defaults and
ordinary-output redaction.

## Executive summary

No exploitable vulnerability was confirmed. Three independent hunters covered
business-logic/time/replay, trust/access/config/redaction, and
wildcard/resource/race classes. Candidate claims were required to include an
executable attack, meaningful impact, and a complete path past existing
mitigations; none met that bar. The slice remains intentionally non-production:
ordinary daemon configuration cannot assert private-LAN proof scope, and no
production host/proof adapter exists.

## Findings

| Severity | Finding |
|---|---|
| — | No confirmed findings |

`findings.json` is an empty, schema-valid list. With no candidate finding,
Phase 3 adversarial validation and Phase 6 per-finding verification had no
items to dispatch.

## Hardening notes

- The source-attribution guarantee ultimately depends on the future protected
  host adapter. Its authentication, host-key pinning, exact dial semantics and
  proof-install authorization require a fresh security audit when composed.
- Real routed-host timing, partition, DNS and publication withdrawal remain R3
  evidence. Local fakes cannot establish hostile-network behavior.
- Additional independent audit runs may explore different paths; one run is
  not a completeness guarantee.

## Positive patterns

- Strict bounded manifest parsing and complete admission precede mutation.
- Safe service default is `outbound_only`; incomplete private-LAN scope fails
  before Waku startup.
- Proof binds exact manifest digest, target, allowed manifest source, address
  and a finite observation window.
- Withdrawal truth is monotonic: older/equal success cannot resurrect an
  endpoint after failure, and ambiguous proof application is compensated by a
  required withdrawal.
- Stop/start and expiry discard proof state; expiry notifies discovery
  publication observers even when first detected by an endpoint read.
- Private, mapped, link-local, loopback, public, DNS, relay and
  profile-incompatible address shapes fail closed.
- Ordinary results retain only bounded counts and stable reason codes; host,
  address, identity, proof and adapter-error material remains protected.

## Dynamic evidence

- Focused Waku/deployment tests: PASS
- Focused race tests: PASS
- Full `go test ./... -count=1`: PASS
- `go vet ./...`: PASS
- `govulncheck ./...`: no called vulnerabilities

