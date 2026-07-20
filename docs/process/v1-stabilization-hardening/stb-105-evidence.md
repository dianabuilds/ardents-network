# STB-105 Evidence — Key And Persistent-State Security

Date: 2026-07-19

## Security Inventory And Contract

`docs/persistent-state-security.md` now defines the canonical inventory and
lifecycle for:

- Ardents Ed25519 identity keys and their public record in `ardents.db`;
- Waku secp256k1 transport keys and Waku Store continuity;
- encrypted blob bytes, metadata, external payload keys, and future capability
  material;
- local API and TCP-WSS deployment secrets;
- diagnostics and other retained product state;
- stopped-node consistency groups, restore acceptance, rotation, revocation,
  and unrecoverable-state behavior.

DEC-STB-005 records the decision to keep decrypting/key material separated from
retained payload/state and to reject partial continuity sets rather than create
plausible replacement identities.

## Runtime Changes

- Standalone secret and payload writes now use same-directory temporary files,
  private permissions, flush, close, and atomic replacement.
- Private directories are `0700` and files are `0600` on Unix. Key reads reject
  group/other access and non-regular paths.
- Windows uses protected DACLs granting access only to the current service
  identity and Local System, with private inheritance for data-directory
  children. A test verifies that `Everyone` and built-in `Users` are absent.
- Existing non-key bbolt, Waku SQLite, blob, and diagnostics files are tightened
  before use. Existing key files remain fail-closed rather than silently
  accepting unsafe or malformed continuity material.
- Identity startup validates Ed25519 private/public agreement and derived
  principal/device identity. Missing, partial, corrupt, or mismatched state/key
  sets stop startup and preserve the retained identity record.
- Identity creation writes the private key before publishing its matching state,
  so an interrupted first creation cannot later rotate silently.
- Waku startup checks whether persistent Store state already exists before key
  creation. A missing Waku key beside an existing Store is fatal and requires a
  matching backup restore.
- Waku Store and bbolt keep their mature transactional engines; no custom
  database or network substrate was introduced.
- Diagnostics retains recursive redaction for secret, token, private key,
  payload/plaintext, ciphertext, nonce, seed, and key material fields.

## Recovery And Negative Evidence

Automated coverage proves:

- a clean restart restores the same Ardents principal and device;
- a stopped full data-directory backup restores the same identity and Waku
  continuity state;
- `ardents.db` without its matching identity key fails closed and does not
  overwrite the retained principal;
- key-only identity state, mismatched public/private keys, invalid key length,
  corrupt key JSON, and missing transport keys beside retained Store state are
  rejected;
- atomic replacement exposes only complete final bytes and leaves no temporary
  file;
- non-regular private state is rejected;
- Waku Store is a private regular file;
- old diagnostics and payload files are migrated to the protected file policy;
- canonical reports contain none of the controlled secret values used by the
  redaction and API-boundary tests.

## Validation

- `go test ./...`: passed after final ACL/persistence changes.
- `go vet ./...`: passed.
- code-size guard across Persistence, Identity continuity, Network transport and
  participation, Data, and Diagnostics recorder: passed with no soft/hard
  breach.
- focused unit and integration recovery/security tests: passed.
- final canonical full runner:
  `tests/run.ps1 -Suite all -ReportDir tests/.artifacts/reports/stb-105-all-final`:
  112 passed, 0 failed, 112 raw reports, exit code 0, 487.2 seconds.
- reports:
  `tests/.artifacts/reports/stb-105-all-final/summary.json` and
  `tests/.artifacts/reports/stb-105-all-final/junit.xml`.
- catalog validation: 112 tests, 26 scenarios, 112 formal bindings, 0 missing
  bindings, 0 missing docs, 0 scenarios without tests, 0 issues.
- controlled-value scan of the final report directory: 0 secret hits.
- `govulncheck ./...`: unchanged and aligned with
  `docs/security-exceptions.md`: one reachable `GO-2026-4479` DTLS v2 finding
  with no upstream fix, contained by supported TCP/TCP-WSS profile suppression;
  zero other reachable/package findings and one unrelated module-only finding.

## Runtime Security Guard Assessment

- Assets: Ardents identity key, Waku node key, API/TCP-WSS secrets, payload and
  future capability keys, retained ciphertext, product state, and diagnostics.
- Invariants: no key/payload co-location, no silent identity rotation, private
  atomic files, transactional databases, explicit recovery, and no secret
  disclosure through control/evidence surfaces.
- Failure posture: fail closed for key continuity and product-state corruption;
  operator-visible recovery for diagnostics and missing payload availability.
- Result: passed on the supported Windows path. Unix permission behavior is
  implemented in platform-specific code and compiled by the same package
  boundary; cross-platform CI remains a later release requirement.

## Gate Result

Passed. Persistent-state defaults are protected and failure paths are explicit;
security scan and exception truth agree; no remediable reachable high/critical
finding remains; all Phase 0 gates are green after the final change.
