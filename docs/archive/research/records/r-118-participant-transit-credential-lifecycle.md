---
id: R-118
title: Participant transit credential lifecycle
status: decided; implementation-linked
owner: Product Owner and Codex
started: 2026-08-26
reviewed: 2026-08-26
---

# R-118 — How can a participant Endpoint obtain, use, renew, and erase the fresh TLS credential paired with each State-authorized Transit Grant without an operator route plan, a hidden online registrar, or Target disclosure?

## Decision this unlocks

Select the owned participant-runtime boundary required for `name.ard` to open
an actual Service through the accepted H4-3/H4-4 path. It must give the
Endpoint a current State view, imported Entry contact, and the grant/key pairs
needed for each private lookup and Introduction submission. It must not make a
JSON plan containing a Target, Descriptor, Gateway, C-2 peer, or private key
into a second routing or naming authority.

## Current contract

- ADR-0039 selects a State-authority-signed, one-use Transit Grant whose tuple
  binds Network, Epoch/Digest, attachment, exact Node/role, expiry, and TLS
  client-key digest. Receiving Nodes verify it offline and durably spend its
  Grant ID.
- `endpoint.transitClientCertificate` accepts a decodable grant only when the
  Endpoint already owns the matching locally enrolled TLS key; it rejects a
  supplied or freshly generated replacement key.
- The H4-4 `OpenAlphaBrowserRuntime` derives the Name Target exclusively from
  the accepted alpha corpus and derives Gateway/Initiator/Introduction/
  Rendezvous only from State plus Entry. It deliberately has no Grant or TLS
  key input.
- The current C-2 process fixture provisions synthetic finite grant/key pairs
  before the route. It is test evidence, not a participant credential owner.
- H4-6A's disclosure catalog is an inspection index; it cannot issue a grant,
  transport private key material, or substitute for live State/control.

## Hypotheses

- **H1:** an Endpoint can create a fresh local TLS public key and obtain an
  opaque State-authority grant bound to an already selected attachment through
  a bounded, privacy-preserving participant credential protocol; it retains
  only the matching key until use/expiry and has no Target input.
- **H2:** a finite explicitly enrolled batch of key/grant pairs can support a
  closed alpha process without a per-request issuer, provided exhaustion,
  withdrawal, rotation, loss, and durable erasure are explicit and no claim of
  general availability is made.
- **H0:** neither option can preserve route privacy, State authority, and
  operational viability for the actual alpha team. In that case H4-4 remains
  a qualified composition trace and must not expose Browser Entry readiness.

## Evaluation criteria

- A browser-demanded name reaches only the Target from the accepted corpus;
  no credential request includes that name, Target, Publication, Descriptor,
  HPKE plaintext, or full route.
- The issuer and receiving Node each see only the minimum adjacent-hop facts.
  The issuer cannot choose a Gateway, C-2 peer, or fallback.
- A grant/key pair is single-use, attachment-bound, time-bounded, restart-safe,
  erased after spend/expiry, and cannot be substituted or replayed after Node
  restart.
- State expiry, authority rotation, depleted batch, issuer withholding,
  private-key loss, malformed response, and endpoint crash all fail locally
  and leave no browser route or fallback.
- The flow has explicit operators, availability, storage, bandwidth, and
  renewal limits suitable for one-person closed alpha operation. It makes no
  permissionless admission, anonymity, or independent-control claim.

## Evidence plan

### Primary sources

- ADR-0039 and R-109, inspected 2026-08-26.
- `internal/route/transit_grant.go`, Node grant spending, and
  `internal/endpoint/transit_client_certificate.go`, inspected 2026-08-26.
- H4-2, H4-3, H4-4, and H4-6 contracts, inspected 2026-08-26.

### Experiment

The first disposable data-flow experiment is
[`experiments/r-118-private-transit-issuance`](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-118-private-transit-issuance/).
It has no network transport: it tests the minimum issuer transcript and exact
Grant/key/one-use tuple before an H4-2 transport is selected. Its strict
request grammar must refuse a trailing Target, a changed State Node, an expiry
longer than State permits, a replacement TLS key, and replay.

The second disposable experiment,
[`experiments/r-118-credential-relay`](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-118-credential-relay/),
now runs separate Endpoint, issuer, and Initiator child processes. It captures
only terminal disclosure/tuple evidence. Its next successor must substitute a
real Entry attachment and State-selected issuance duty, then prove successful
private lookup and C-2 opening while rejecting changed attachment/key/role/
Node/Epoch/Digest, replay after restart, key loss, expired/depleted/withheld
issuance, and a request containing a Target or Descriptor. Retain these
experiments only if they support a selected maintained boundary.

### Failure scenarios

- A credential issuer links a name/Target to a User or learns Publisher data.
- A stale or duplicate grant/key pair is accepted by an active or restarted
  Node.
- A descriptor causes the Endpoint to obtain a credential for a substituted
  Introduction or a Grant issuer selects a C-2 peer.
- A project operator manually approves individual name opens or silently
  supplies a route plan.
- A stranded pre-issued batch becomes a hidden availability promise or its
  private material survives Endpoint removal.

## Findings

- **Current-code fact:** the Transit Grant contains `AttachmentID` and the
  digest of the client TLS public key. The Endpoint cannot know a valid grant
  before it chooses the fresh attachment/key pair, and cannot replace the key
  after receiving a grant.
- **Measured maintained-fixture fact (2026-08-26):** the separate-process
  C-2 tracer creates three real State-authority-signed grants, each paired
  with a separately generated local TLS certificate.  Its User receives the
  one Introduction pair and uses the exact fixed `serviceAttachment` that the
  signed grant names.  This proves the receiving Node's signed-grant/key
  mechanics; it is a one-user, preassembled route fixture, not credential
  issuance or participant enrollment.
- **Current-code fact (2026-08-26):** `OpenAlphaBrowserRuntime` creates a
  fresh private-lookup attachment for each browser demand.  For C-2, it now
  verifies a decodable Introduction Grant against the current State authority
  and uses that Grant's exact attachment; absent a Grant grammar it retains
  the fresh-key compatibility path. Its end-to-end browser tracer now carries
  a real signed Grant, a matching Endpoint-local certificate, and a receiving
  Introduction admission that verifies the signed tuple. This proves that the
  State-driven runtime does not substitute the Grant attachment or key. The
  test fixture still directly provisions that pair, so it does **not** prove
  participant issuance, secure key delivery, persistence, or renewal.
- **Current-wire fact (2026-08-26):** one Reachability Descriptor exposes one
  `SubmissionAuthorization`.  A finite batch of distinct user grant/key pairs
  cannot make that single field select a pair for two independent Endpoints.
  Reusing a shared client key would violate the one-use local-key premise;
  adding a caller plan would create the rejected second route authority.
- **Current-wire measurement (2026-08-26):** a Transit Grant v1 is 332 bytes
  (`25`-byte prefix, fixed 268-byte signed body, and 64-byte Ed25519
  signature). The Descriptor authorization limit is 1,024 bytes. Even a new
  compact set grammar could therefore carry at most three full grants, before
  it carries its own version/count fields. That cannot provide a renewable
  per-attempt batch for two independent alpha Endpoints; one spent Grant is
  not reusable after a Node restart.
- **Contract consequence:** ADR-0039 deliberately accepts a finite pair
  provisioned to one intended Endpoint at publication time. It proves a
  closed one-user trace, but does not itself meet H4-4A's two-independent-
  Endpoint or repeatable browser-use outcome. The missing selection/issuance
  owner is consequently a real product and protocol gap, not a Firefox or
  localhost-presentation defect.
- **Current-code fact (2026-08-26):** `entry.Owner.Acquire` can open one
  State-pinned adjacent Initiator connection using an opaque Invite and a
  fresh client key, but it has no credential-issuer request grammar or user
  identity API. Reusing it for issuance would require a distinct selected
  request/domain and explicit admission semantics; it must not reinterpret an
  Invite as a general issuer credential. The existing OHTTP clients likewise
  implement only their named resolution messages, not a generic control
  channel.
- **Measurement (2026-08-26):** all six cells in
  [private Transit issuance experiment](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-118-private-transit-issuance/) passed locally. The positive
  issuer transcript contained only Network ID, State digest/Epoch, transit
  Node/role, attachment, client-key digest, and expiry; it contained neither
  the synthetic Target nor `reference.ard`. Strict decoding refused an
  appended Target, a changed Node, and an expiry longer than State permits;
  the synthetic Node refused a substituted TLS key and a second Grant spend.
  This demonstrates grammar feasibility, not transport privacy, participant
  admission, durable persistence, or operational availability.
- **Measurement (2026-08-26):** all six cells in
  [credential relay experiment](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-118-credential-relay/) passed locally under an explicit
  build-ignored experiment source. It starts separate Endpoint,
  Initiator, and issuer child processes. In its exact cell, the issuer saw the
  Initiator's adjacent connection, received no forwarded synthetic admission
  proof, and decapsulated a fixed-capacity tuple containing none of the
  synthetic Service Name, Target, or Descriptor markers. A second use of the
  one local admission was refused by the Initiator before forwarding; changed
  Node and expiry were refused by the issuer, and the Endpoint rejected a
  signed response with a changed client-key digest. The invalid-target cell
  intentionally lets the issuer observe the trailing Target only to prove
  strict rejection; it is not a successful-flow disclosure claim.
- **Inference:** the selected Entry-plus-OHTTP carrier form can carry the
  candidate exact issuance transcript without making an issuer the Endpoint's
  adjacent peer or forwarding the admission proof. This is conditional on an
  honest Initiator and a synthetic one-use admission in the experiment. It
  does not prove resistance to Initiator/issuer collusion, a global issuance
  budget, real Entry TLS composition, State selection of an issuer, or a
  participant process lifecycle.
- **Current-code fact (2026-08-26):** `route.OpenEntryAttachment` can open a
  real State-pinned Entry TLS attachment from the imported `entry.Owner`, and
  `route.AcceptEntryAttachment` durably consumes that attachment through the
  Initiator's `entry.Admitter`. The fresh Entry TLS certificate is local to
  that carrier and need not be the separate TLS key whose digest the requested
  Transit Grant binds; the latter can remain Endpoint-local inside the OHTTP
  tuple. This removes no control requirement, but shows that the candidate
  does not need to expose a Transit private key to Entry in order to use real
  Entry admission.
- **Current-code fact (2026-08-26):** `node.Initiator` admits exactly a native
  C-2 `RelaySetup` or `ResolutionRelaySetup`. The latter is hard-bound to the
  sole `destination-resolution` State Gateway. It cannot carry issuance merely
  by changing a URL or profile. A real integration therefore requires the
  separate `CredentialRelaySetup` and State-selected issuance duty described
  below, with dedicated admission-budget and withdrawal semantics. The
  existing Entry ledger's finite attachment replay capacity is not such a
  budget; it is deliberately only a per-attachment replay guard.
- **Measurement (2026-08-26):** the build-ignored
  [Entry carrier logic prototype](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-118-entry-carrier/) passed. It opened a real
  imported `entry.Owner` attachment through `route.OpenEntryAttachment` and
  accepted it through `route.AcceptEntryAttachment` plus a separate durable
  `entry.Admitter`. Its post-admission handler received one opaque blob and no
  Invite marker. The experiment establishes that an issuer-request carrier can
  use the existing Entry TLS admission form while keeping its separate Transit
  TLS key local to the Endpoint; it does not create the new operation or prove
  a selected issuer/durable issuance budget.
- **Current-code fact (2026-08-26):** an Introduction Node consumes a Transit
  Grant only as its exact adjacent-hop admission. It separately matches the
  sealed introduction against the Publisher-registered Reachability and
  JoinHandle. The Grant therefore does not itself authorize one Service or
  Name. This agrees with the H4-4 product contract: network naming/discovery
  is not Service authorization; the Publisher Application remains responsible
  for that decision.
- **Current-code fact (2026-08-26):** Descriptor v1 has exactly one fixed
  `SubmissionAuthorization`; the browser runtime treats any decodable Grant
  there as the sole exact attachment/key pair it may use. Descriptor v2 is a
  separate signed declaration with no embedded authorization. It must obtain
  one fresh Grant through the State-selected Credential Relay and must never
  reinterpret v1 as that permission.
- **Maintained behavior measurement (2026-08-26):**
  `TestAlphaBrowserRuntimeIssuesMembershipGrantThroughStateCredentialRelay`
  passed locally. It publishes a Descriptor v2, projects one signed issuer
  profile through the State-view port, creates a fresh Endpoint TLS key and
  attachment, consumes a distinct Entry attachment for exactly one Credential
  Relay exchange, verifies the resulting State-authority Transit Grant, and
  completes the existing Introduction/Responder/Publisher HTTP flow. The
  issuer authorization saw only Network, epoch/digest, Introduction identity,
  attachment, TLS-key digest, and short expiry; the test asserts the latter
  two bindings are non-zero. The test State view and project-operated issuer
  callback are controlled behavior evidence, not a durable participant
  lifecycle or hostile-peer membership proof.
- **Inference:** adding Gateway, peer, Descriptor, grant, or TLS-key fields to
  an `endpoint run` plan would make an unaudited operator input a second route
  authority. It is not an H4-4 completion mechanism.
- **Current-code fact (2026-08-26):** the maintained issuer TLS listener
  requires a client certificate whose Ed25519 Node key is the exact selected
  Initiator key; a direct client without that certificate is refused during
  TLS. The Initiator pins the issuer certificate to State, rejects a setup
  whose issuer-profile digest differs from its State projection, and the
  issuer rechecks a typed current-State duty before each signature. These are
  confinement checks, not a durable issuer process or hostile-peer admission
  mechanism.
- **Inference:** H4-6A disclosure cannot solve this: it is intentionally
  non-authorizing and has no private Endpoint material. A later issuer design
  must state whether it is an H4-2 route capability, H4-6 control capability,
  or a bounded alpha enrollment batch.

## Options

1. **Fresh bounded credential issuance.** The Endpoint locally creates an
   attachment/key pair, then obtains a signed exact grant through a selected
   protocol that reveals no Name/Target or Publisher material. This aligns
   with one-use Grants but requires a new issuer availability/privacy contract.
   Its request can be limited to State epoch/digest, the already selected
   Introduction Node/role, attachment, key digest, expiry, and an alpha
   enrollment admission proof; it must never carry Name, Target, Descriptor,
   Publication, JoinHandle, or sealed introduction bytes.
2. **Finite enrolled grant/key batch.** A release/enrollment-bound artifact
   delivers a small opaque batch for declared alpha work. It is simpler but
   requires a protocol-defined, Descriptor-authorized way for the Endpoint to
   select its own pair (and use that grant's exact attachment); the current
   single authorization field cannot do this for multiple users. It also has
   exhaustion, private-key delivery, revocation, and operational-control
   costs; it cannot become a general browser-ready claim.
3. **Operator plan or static callbacks.** Reject. It repeats the fixture
   problem identified by R-109 and violates H4-4's State/Entry-only route
   authority boundary.
4. **Fresh unauthenticated TLS key for a signed Grant.** Reject. The Grant
   deliberately binds the key digest and Endpoint rejects this downgrade.

5. **Versioned membership-level dynamic submission.** A new Descriptor
   authorization grammar explicitly says that its Introduction submission
   requires a fresh State/Entry-issued membership Grant, rather than embedding
   one fixed Grant/key pair. The Endpoint obtains that exact Grant through the
   selected Credential Relay and presents it to the selected Introduction;
   the Descriptor still supplies only the signed Introduction, Reachability,
   JoinHandle, and expiry facts. The issuer sees no Name/Target or sealed
   bytes. This preserves the product rule that the network is not a
   Service-authorization layer. It is the only candidate consistent with
   repeatable independent alpha Endpoints, but it requires a new Descriptor
   version, State-selected issuer duty, and explicit alpha admission budget.
6. **Publisher-delegated dynamic ticket.** A Descriptor would carry a
   publisher-specific issuer ticket in order to request a fresh Grant. Reject
   for alpha unless a later product requirement needs network-level
   Service admission: it creates a second Publisher-to-control authorization
   protocol even though the sealed JoinHandle/Application already controls the
   Service path.

### Alpha admission-control boundary — selected alpha limit

The issuer cannot independently prove that an opaque request arrived after a
valid Entry admission unless it receives either a stable Entry-derived fact or
a proof minted by the selected Initiator. A raw Invite/Invite ID would give the
issuer a stable participant correlator; an Initiator-signed one-use proof hides
that fact but a malicious selected Initiator can mint arbitrary proofs. Blind
admission credentials could address this distinction, but introduce a new
cryptographic/control protocol and are not a consequence of H4-4.

There are therefore two honest choices:

1. **Project-operated alpha:** State selects and discloses the Initiator and
   issuer; they are trusted to enforce one issuance exchange per accepted Entry
   attachment and their own bounded duty resources. The issuer has no User
   admission or per-Invite quota claim, and the alpha makes no claim that it
   withstands a malicious selected Initiator or issuer.
2. **Hostile-peer membership control:** require an issuer-verifiable,
   unlinkable, rate-limited Entry-derived credential. This is a separate H4-6
   control/cryptographic research decision; do not silently describe the alpha
   form as achieving it.

The first is the smallest form compatible with H4-6A's explicit
project-operated alpha. The second is not a prerequisite for an honest alpha,
but is a prerequisite for a claim that arbitrary participating Nodes cannot
mint admission for one another.

### Selected H1 transport

The only current candidate is a new **Credential Relay** operation patterned
after ADR-0037, but with a separate State-selected `transit-issuance` duty and
message domain:

1. The Endpoint creates one fresh attachment and TLS key after it has current
   State, and opens a fresh Entry attachment distinct from lookup and C-2.
2. A `CredentialRelaySetup` binds only Network, State digest/Epoch, the
   admitted Initiator, one State-selected issuance duty and exact signed
   profile digest, deadline, and fixed ciphertext capacity. It carries no
   Name, Target, Descriptor, or C-2 peer.
3. The Initiator verifies that exact State selection and forwards one opaque
   OHTTP envelope over TLS authenticated with its Node certificate. It cannot
   decrypt the issuance request or choose another destination. The issuer
   requires that certificate and accepts only the current State-selected
   Initiator as its adjacent peer.
4. Inside the envelope, the request uses the R-118 experiment's fixed tuple:
   Network, State digest/Epoch, Introduction Node/role, attachment, TLS
   public-key digest, and bounded expiry. The issuer rechecks the duty against
   its current State before signing; it cannot select a Gateway, C-2 peer, or
   fallback.
5. Endpoint verifies that the returned Grant exactly matches its locally
   retained key and requested tuple, persists the pair until one spend/expiry,
   then erases it on either outcome.

This selected project-operated-alpha form introduces an online State-authority
signing operation and therefore explicit H4-2/H4-6 availability, budget,
key-custody, withdrawal, and operator obligations. Its maintained
implementation must prove that Initiator limits one exchange to one accepted
Entry attachment without disclosing the Invite or a stable User identity to the
issuer. It does not claim that this remains sound with a malicious selected
Initiator or issuer.

## Recommendation

Choose versioned membership-level dynamic submission for the
project-operated alpha. The static batch is rejected for repeatable
multi-Endpoint use. The selected maintained successor is a State-selected
Credential Relay with real Entry admission, a bounded per-attachment exchange,
and a State-authority Transit Grant v1 response. It must use a versioned
Descriptor declaration and must never reinterpret a fixed Grant as a request
for a new one. Confidence is medium: the transcript and carrier are
reproducible, but the honest-operator admission/budget limit is explicit and
hostile-peer membership control remains an H4-6 question.

## Disposition

Decided on 2026-08-26: ADR-0047 accepts versioned membership-level dynamic
submission and the project-operated-alpha Credential Relay limit. The
maintained descriptor, State projection, Entry-carrier, issuer, Initiator, and
Endpoint behavior tests now cover the selected success path and exact tuple
refusals. `ardents endpoint alpha-browser` additionally owns the already
accepted State, Entry, and corpus roots while the named runtime is live; when
the matching bounded Source plan is present, it also owns the initial State
wave and automatic refresh for that root. Its input cannot carry a route or
Transit credential. This does not close durable issuance budgeting,
withdrawal/rotation, crash retention of a pending grant, multi-host/expiry
qualification of that refresh, or hostile-peer membership control. This record supplies
implementation evidence, not a second protocol specification. Retain the
experiments as provenance until the H4 cleanup decision records whether their
distinct negative evidence is duplicated by maintained tests.
