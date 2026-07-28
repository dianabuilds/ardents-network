# PW3-13: CGA-03 rotate a channel and attest activation

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R1 multi-host lifecycle with security review

## Parent

`../PRD.md`

## What to build

Rotate one channel end to end: create a fresh pending generation, deliver it to
every approved member, append the activation checkpoint/envelope, accept
active receipts, and retain the immediately previous generation as
receive-only for a bounded drain. Restarts resume the same phase and operation.

## Acceptance criteria

- [x] Rotation always creates a fresh selector/key and never publishes with
      the previous generation after activation.
- [x] Current, pending and one receive-only previous generation have explicit,
      bounded, versioned member and authority state.
- [x] A member missing the activation checkpoint is not ready even if it has
      installed the pending secret.
- [x] Crashes at every ledger/checkpoint/delivery/activation boundary resume
      one operation without a second generation or audit identity.
- [x] Suspect or noncompliant members require deployment fencing; a valid MAC
      alone cannot establish honest activation.
- [x] Only one pending generation is admitted and post-activation recovery
      rolls forward rather than rolling authority sequence back.
- [x] Redaction, stable errors, restart, race and multi-host integration checks
      pass without changing `Q`.

## Blocked by

- PW3-12 / CGA-02 accepted with commit-bound evidence.

## Comments

- Published as the canonical successor slice. It remains `needs-info` until
  CGA-02 is accepted.
- 2026-07-28 predecessor gate satisfied: the maintainer explicitly accepted
  CGA-02 implementation commit
  `693ac7cb0e88661dccce8a97482ae14d53a5afd9`. CGA-03 is admitted for
  implementation; no CGA-04 behavior, qualification, deployment change or push
  is implied.
- 2026-07-28 CGA-03 implementation handoff:
  - exact starting commit:
    `89db8079920dbb8086adfc32ee295fc3c9865abf`;
  - exact implementation tip:
    `34dafff129e4dc26fe42932946df566e0295c84d`;
  - implementation commits:
    `00dc538`, `18527e4`, `fcdf198`, `34dafff`;
  - protected Operator-only rotation now spans fresh pending generation,
    per-member sealed delivery/install acknowledgement, signed activation,
    live member runtime adoption and deployment-approved active receipts.
    Exact channel, operation and delivery resources have no Application
    equivalent;
  - the stable capability reference switches publish to the fresh generation
    only after activation. The immediately previous generation is
    receive-only until the bounded drain; discovery, Store and live transfer
    consumption cover current plus previous topics;
  - installed pending state and committed-but-not-runtime-adopted state are
    explicitly not ready. A failed live resubscription retains the old
    subscription and withholds the active receipt until a successful durable
    adoption retry;
  - operation-keyed activation history preserves replay across later
    strictly-monotonic rotations. Authority crash injection covers both
    ledger/checkpoint crash points for rotate, installed acknowledgement,
    activation commit and active acknowledgement without creating a second
    operation, generation or audit identity;
  - a valid receipt MAC with `approved_host=false` is rejected. The CLI
    requires explicit deployment-owned `--host-disposition approved`;
    CGA-04 remains responsible for real fencing evidence and membership
    mutation;
  - rotation reserves the full `2 * member_count + 2` audit/audit-outbox
    budget before pending state and checks operation/rotation bounds, so an
    admitted operation cannot be stranded at the exact permanent limits;
  - commit-bound validation passed:
    - `go test ./... -count=1`;
    - `go vet ./...`;
    - `go test -race ./internal/authority
      ./internal/identity/capability ./internal/messaging
      ./internal/discovery ./internal/transfer ./internal/channeldelivery
      ./internal/localapi/authority ./internal/localapi/channeldelivery
      -count=1`;
    - `go test -tags=integration ./tests/integration/localapi -count=1`;
    - `scripts/generate-api.ps1 -Check`;
    - `go run ./tests/tooling/capabilitycatalog -check`;
    - `git diff --check 89db807...HEAD`;
  - protected two-host integration, consecutive rotation, live pre-started
    resubscription failure/retry, exact-bound, negative authorization,
    redaction and restart cases are retained in the committed suites;
  - independent repeat Standards and Spec reviews both returned `PASS` with
    no remaining actionable findings;
  - the capability catalogue remains `24 capabilities, 8 domains, 0
    qualified`; canonical qualification is unchanged and `Q=no`;
  - CGA-04 remains gated on explicit maintainer acceptance of the exact
    implementation tip. No production deployment was changed and nothing was
    pushed.
