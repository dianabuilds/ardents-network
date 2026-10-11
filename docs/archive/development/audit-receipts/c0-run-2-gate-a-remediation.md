# C0 deep-audit run 2: Gate A remediation receipt

Status: **Gate A accepted; Track B not started**

This receipt closes the source-artifact Gate A findings recorded by the
immutable [run-2 Track A receipt](c0-run-2-track-a.md). It records a bounded
source-audit verdict for one exact candidate. It is not release Qualification,
a platform or deployment result, or permission to start Track B.

## Identities

- Campaign: `c0-deep-audit-run-2-gate-a-remediation`
- Audit activation profile: `c0-source-artifact-track-a-v1`
- Finding baseline commit: `2e2b63ad2c29d17ffe2f6654f629e79e2de5c909`
- Finding baseline tree: `7ac32cb8478544323ea3cb567156af21cd05f43a`
- Current integration base commit: `78475f0e`
- Accepted candidate commit: `95621daa573517320fcb038b4e5dd0640d80c482`
- Accepted candidate tree: `53a22d59894aa06d6e1e39ba7afdb7b0835581cf`
- Evidence date: `2026-09-02`
- External evidence root: Codex visualization workspace
  `01a05eaf-3683-7202-8b90-939347590838` on the Product Owner Windows host.
- Absolute locator at capture:
  `C:/Users/vitek/.codex/visualizations/2026/09/01/01a05eaf-3683-7202-8b90-939347590838/security-audit/ardents-network/gate-a-remediation/95621daa573517320fcb038b4e5dd0640d80c482/`
- External `evidence-index.json` SHA-256:
  `aee5b98d7372a9cabbcf3fa4cd763cafab130cb9c16a3369f51efd57273b9363`

The receipt is committed after the accepted candidate, so the receipt commit is
not the source candidate identified above.

## Cumulative diff and Track A change impact

The cumulative baseline-to-candidate diff contains 186 paths, 2,567 insertions,
and 10,311 deletions. The deletion-heavy shape is expected: ADR-0069 retires
the active Browser implementation and completed local ceremony surfaces rather
than preserving obsolete runtime identities.

Exactly 67 of the 157 Track A cells were affected and re-reviewed. The other 90
cells remained eligible for strict reuse because their owned paths and authority
inputs were unchanged by the cumulative diff. The affected cells were:

- Browser (15): `BR-ADAPTER`, `BR-CMD-ADAPTER`, `BR-CMD-ENTRY`,
  `BR-ENTRY-ENROLLMENT`, `BR-ENTRY-INSTALLER`, `BR-ENTRY-STATE`,
  `BR-FIREFOX-SOURCE`, `BR-PACKAGE-BUNDLE`, `BR-PROFILE`,
  `BR-QUAL-SIGNED-XPI`, `BR-QUAL-UBUNTU`, `BR-QUAL-WINDOWS`,
  `BR-REFERENCE-PROXY`, `BR-REFERENCE-STATIC`, and
  `BR-REFERENCE-TRANSPARENT`.
- Control (13): `CTL-ALPHA-02`, `CTL-ALPHA-05`, `CTL-ALPHA-07`,
  `CTL-CMD-01`, `CTL-DUTY-18`, `CTL-E2E-30`, `CTL-RELEASE-CMD-15`,
  `CTL-RELEASE-CUSTODY-16`, `CTL-STATE-GENESIS-17`, `CTL-XADR-36`,
  `CTL-XADR-38`, `CTL-XDOC-34`, and `CTL-XDOC-35`.
- Coordinator (25): `COORD-ADR-06`, `COORD-ADR-10`, `COORD-ADR-12`,
  `COORD-ADR-13`, `COORD-ARCH-31`, `COORD-ARCH-32`, `COORD-ARCH-33`,
  `COORD-ARCH-34`, `COORD-AUDIT-15`, `COORD-BUILD-05`, `COORD-DEPS-04`,
  `COORD-DEV-14`, `COORD-DOMAIN-03`, `COORD-OWN-16`,
  `COORD-PACKAGE-MAP-17`, `COORD-PRODUCT-20`, `COORD-PROFILE-37`,
  `COORD-REF-23`, `COORD-ROOT-01`, `COORD-SECURITY-25`,
  `COORD-TECH-CONTROL-26`, `COORD-TECH-ENDPOINT-27`,
  `COORD-TECH-NAMING-28`, `COORD-TECH-NETWORK-29`, and
  `COORD-TESTING-18`.
- Endpoint (6): `E-APP-01`, `E-E2E-01`, `E-END-01`, `E-XADR-02`,
  `E-XDOC-01`, and `E-XDOC-02`.
- Routing (8): `RT-CRED-01`, `RT-CRED-02`, `RT-NODE-02`, `RT-NODE-06`,
  `RT-ROUTE-01`, `RT-XADR-04`, `RT-XDOC-01`, and `RT-XDOC-02`.

New ADR and test paths were assigned to the corresponding existing authority or
implementation cells. Completed research records and the earlier receipt remain
historical provenance and were not treated as current product contracts.

## Finding dispositions

| Finding | Disposition |
| --- | --- |
| `A-COORD-01` | Closed. Source-audit activation and Gate A are explicitly separated from release Qualification; current owner documents no longer promote an audit run into a release claim. |
| `A-COORD-02` | Closed with deterministic proof. Canonical Go builds force `-trimpath -buildvcs=false`; repository-representation tests cover normal-clone and linked-worktree inputs. |
| `A-BR-01` | Closed by retirement. The Browser qualification lane that depended on Network commands omitted by Application extraction is no longer active. |
| `A-BR-02` | Closed by retirement. The duplicate persisted Browser enrollment grammar is absent from maintained runtime code. |
| `A-BR-03` | Closed by retirement. The Reference Site no longer derives process lifetime from the first HTTP request context. |
| `A-BR-04` | Closed by retirement. The unbounded Browser runtime and challenge state are absent from maintained runtime code. |
| `A-BR-05` | Closed by retirement. The native-manifest installer and its failure residue are no longer an active delivery surface. |
| `A-BR-06` | Closed by retirement. The target-bearing Reference implementation without a non-test caller is absent from maintained code. |
| `A-BR-07` | Closed. ADR-0069 records Browser retirement and current product, technical, ownership, profile, and command documents consistently describe Browser work as retired evidence or a future selection. |
| `A-END-01` | Closed with proof. Admission expiry is separated from an already accepted Application Connection lifetime; active connections are not terminated merely because the admission TTL elapsed. |
| `A-END-02` | Closed with proof. The Broker enforces the selected 64 active-Connection capacity independently from administration activity. |
| `A-CTL-01` | Closed. Alpha-control duty documentation assigns and describes the receiving replay ledger consistently with maintained implementation ownership. |
| `A-CTL-02` | Closed. Production readers no longer export test-only signing or writer authority; focused test owners keep local fixtures without creating a speculative shared package. |
| `A-RT-01` | Closed with failure-path proof. Issuance requires an exact fresh, non-conflicting State duty tuple—generation, Network ID, digest, issuer, epoch, and deadline—before any replay-ledger access; rejected mutations leave the issuer root byte-identical and make zero ledger calls. |
| `A-RT-02` | Closed with concurrent proof. Route and Attachment close operations join one cleanup completion/result instead of racing independent cleanup. |

All 15 recorded findings are closed: 15 closed, zero deferred, zero open.
Browser findings are closed by removal of the active implementation, not by a
claim that a future Browser design has been selected or qualified.

## Verification

Affected implementation cells were exercised with focused package and E2E
commands, including the exact State-duty failure matrix and the architecture
extraction and artifact-representation proofs. The aggregate affected-package
run covered:

```text
go test -count=1 ./internal/architecture ./internal/alphacontrol \
  ./internal/alphacontrol/inspection ./cmd/ardents-control \
  ./internal/application/broker ./internal/endpoint ./internal/network/duty \
  ./internal/route ./internal/route/credential ./internal/node \
  ./internal/network/state ./internal/release ./cmd/ardents \
  ./cmd/ardents-custody ./cmd/ardents-node ./tests/e2e/endpoint
```

The exact candidate then passed:

- `make quick-check`
- `make headless-check`
- `make check`, including staticcheck, vulnerability analysis, unit, build,
  E2E, and race profiles
- independent Standards review: PASS, no actionable findings
- independent Spec review: PASS, all 15 findings closed and no Track B work

An earlier pre-candidate full check exposed one stale unused test helper after
the architecture-test split. That integration defect was removed in the
accepted candidate; the three commands above were repeated on the new exact
commit and tree.

## Artifact identity

Two independent Windows amd64 builds used the canonical
`-trimpath -buildvcs=false` flags and produced byte-identical artifacts:

| Artifact | SHA-256 |
| --- | --- |
| `ardents-control-windows-amd64.exe` | `f9ef563baf8d7d749baeeb6866a505ba9e498789f0bef4109d5851ee9f909eba` |
| `ardents-custody-windows-amd64.exe` | `97392ba3af0dc9e472b5d93a6127808e93bb3b21637f757f968f75ac543fcab0` |
| `ardents-node-windows-amd64.exe` | `5de1e20082518cb44982062ec962abfdaa49b061296fc683f136b989b23af8cf` |
| `ardents-windows-amd64.exe` | `cdbbde6cb7dc0b58d5dfecf44006b1f7724b02555f21de142d5b0d4e938038be` |

The headless alpha bundle also reproduced exactly at
`098678bd13347b8955f2f20812de356c2f7711433c81c7f51de4c52919dbc560`;
its successor bundle was
`efd3a2c65c4ccf07c2789bb657b6a1032d1764f2a47f86a7f2287962314dc551`.
Both independent artifact sets are retained outside the repository beneath the
external evidence locator.

## Gate decision and limits

**Gate A accepted; Track B not started.**

This verdict means the exact source-artifact candidate has no open run-2 Gate A
finding after change-impact review. It does not establish release Qualification,
signed-XPI delivery, public Browser support, a platform matrix, VPS or external
operator behavior, hostile-load resistance, anonymity, availability,
performance, soak, or deployment readiness. Those claims require their own
selected work and evidence; none was started by this remediation.

Any source change after the accepted candidate changes its identity and
invalidates the affected Track A cells and artifact conclusions. The immutable
finding receipt remains unchanged; this receipt records its bounded remediation
outcome rather than rewriting history.
