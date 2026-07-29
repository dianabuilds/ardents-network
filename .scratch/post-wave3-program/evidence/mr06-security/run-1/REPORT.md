# MR-06 security audit — run 1

Target commit: `d05fa68`

Scope: MR-06 logical implementation range `f167e8e..d05fa68`.

## Executive summary

No exploitable vulnerability remains at the target commit.

The audit confirmed one Medium pre-remediation configuration/state-machine
defect: replacing WSS cert/key/CA material behind an unchanged
`certificate_ref` did not change the host configuration digest, so a
controlled restart could be skipped and old in-memory certificate and AutoNAT
truth could remain current. Commit `d05fa68` corrected the defect by binding a
canonical non-secret material-generation digest into preflight, per-host
configuration truth, restart classification and final running-generation
status. Independent Phase 6 verification passed.

A second candidate concerning the WSS validator/library double read was
rejected. Exploitation requires write authority over the protected private-key
file, which already grants equivalent impersonation and denial capability;
untrusted replacement certificates remain rejected by clients.

## Findings at target commit

| Severity | Finding |
|---|---|
| — | No confirmed remaining findings |

`findings.json` is an empty, schema-valid list for the audited target commit.

## Remediated during audit

### Medium — same-reference WSS material rotation could skip restart

Pre-remediation, the host configuration digest included only the certificate
reference and identity. A secret manager could atomically replace the
certificate deployment bundle behind that stable reference while the
coordinator accepted `unchanged`; the existing Waku process would continue
serving its old in-memory certificate and old reachability generation.

The remediation:

- requires a canonical lowercase SHA-256 digest of the validated non-secret
  certificate deployment generation for WSS and forbids it for TCP;
- includes that generation in only the affected host's configuration digest;
- requires `restarted` whenever the generation changes;
- requires protected final status to echo all three exact running
  slot/configuration generations;
- adds malformed-generation, stale-status and same-reference rotation tests.

## Rejected candidate

The validator reads WSS material before go-waku reloads the pair from paths
during node construction. The race is real, but a party able to replace the
permission-protected private-key file already has equal or greater ability to
impersonate the endpoint or deny service. A replacement outside the trusted
chain/SAN remains client-rejected, so the race adds no exploitable privilege or
impact beyond the starting authority.

## Positive patterns

- Strict bounded manifest admission and all public preflight checks precede
  mutation.
- Freshness is rechecked once for every observation immediately before apply.
- TCP cannot acquire certificate truth through an adapter response.
- Address, certificate reference/identity and actual material generation are
  local-host configuration inputs; unrelated host changes do not restart the
  full topology.
- Apply responses bind exact current/prior digests, one closed action and
  identity preservation.
- Final protected status binds the exact manifest and running configuration
  generation before ordinary `ready`.
- Restart creates fresh runtime reachability state; publication remains
  withheld until a new AutoNAT `Public`.
- Ordinary output contains no host, address, certificate, configuration,
  identity, image or signer material.

## Residual assumptions

- Future production adapters must authenticate host/preflight/status truth,
  enforce SSH host-key pinning, calculate the documented certificate
  generation over the same validated bundle and prove the running generation.
- Real WAN dialback, NAT/firewall denial, public/private PKI and rotation remain
  R3 qualification evidence.
- Additional independent audit runs may explore different paths; one run is
  not a completeness guarantee.

## Dynamic evidence

- Focused deployment/Waku tests: PASS
- Focused race tests: PASS
- Full `go test ./... -count=1`: PASS
- `go vet ./...`: PASS
- Tooling and capability catalogue checks: PASS
