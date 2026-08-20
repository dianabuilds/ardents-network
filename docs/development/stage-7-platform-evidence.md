# Stage 7 install, update, principal, and isolation evidence contract

Status: **review behavior inventory. R-049–R-054, exact serialization, numeric
profile, host images, technology candidates, campaign identity, and episode
counts remain open. This document does not authorize a campaign or coding.**

## 1. Verdict meaning and authority split

Stage 7 uses disjoint authorities:

1. `manifest`: immutable profile/schema, source/supply/host/platform identities,
   roots, metadata/artifact commitments, precommitted inputs, clocks, resources,
   faults, expected runtime classes, and cell schedule;
2. `private fixture`: reusable secrets and unpredictable canaries unavailable to
   candidate and verifier until their declared phase;
3. `evidence`: immutable ordered raw platform/candidate/observer observations
   and cleanup inventory produced without verdict authority; and
4. `verdict`: independent deterministic recomputation from the other declared
   artifacts, writing only to a separate root.

The verifier returns:

- `pass`: the artifact set is complete and trustworthy and every required
  predicate matches the declared contract;
- `fail`: artifacts are valid, but at least one candidate behavior breaches a
  functional, security, resource, privacy, platform, or cleanup predicate; or
- `invalid`: schema, provenance, identity, hash, ordering, clocks, observations,
  trust separation, required records, or cleanup integrity is missing,
  ambiguous, contradictory, or contaminated.

An expected runtime rejection such as `release-invalid`, `rollback-refused`,
`authority-locked`, or `isolation-unsupported` can yield verifier `pass`.
Candidate exit code, log text, self-test, installer report, or command summary is
never a verdict.

## 2. Required manifest and evidence fields

R-054 freezes canonical bytes. The behavior schema MUST include at least:

- schema/profile/campaign/run/cell/attempt identity and source commit;
- immutable Ubuntu/Windows image, kernel/build, filesystem/volume, CPU,
  firmware/hypervisor, resource, package-tool, and external-tool identity;
- environment/network/initial-root identity and project-control declaration;
- release metadata role/key/threshold/version/expiry and target commitments;
- artifact length/digest/source/build/dependency/SBOM/attestation/qualification
  commitments and retrieval mode/source schedule;
- installed layout/ACL/mode/owner, activation generation, transaction state,
  journal predecessor, staged/current/rollback payload, schema, and watermark
  commitments;
- Vault/Bundle public parameters and secret commitments without secret values;
- Local Grant, principal start, OS identity, process-tree/job/namespace,
  channel, session, Isolation Context, resource, and deadline commitments;
- isolation profile, allowed IPC/storage, forbidden destination/listener/protocol,
  child/helper schedule, external observer, scan, and packet/DNS capture identity;
- fault point, monotonic clock origin, deadline, expected runtime class, and
  exact required observation predicates; and
- complete created/retained/removed external path, process, socket/pipe, handle,
  job/namespace, rule/profile, package registration, service/start entry, and
  secret cleanup inventory.

Evidence MUST retain native-resolution security/transition events. One-second
resource samples do not replace exact activation, packet, listener, queue, or
escape observations. Candidate self-reported resource or network facts are
diagnostic; controlled platform/host observers are authoritative.

## 3. Mandatory behavior matrix

Every row is required on each applicable frozen platform. Unless stated,
correct handling has expected verifier result `pass`. A valid artifact set with
different behavior is `fail`; incomplete/untrustworthy artifacts are `invalid`.

### A — Install, environment, repair, remove

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| A0 | Clean offline install from valid untrusted-channel package | Exact environment/root/platform installed; unprivileged Endpoint; no account/Authority/Service/Node/remote listener; explicit offline readiness |
| A1 | Wrong environment/network/root/platform/architecture/package identity | `release-incompatible` or `release-invalid`; no owned state mutation |
| A2 | Package signature valid but Ardents target unauthorized, or reverse | Unauthorized Ardents bytes never execute; channel-signature failure remains separate and explicit |
| A3 | Repair valid immutable files/registration | Restore exact install artifacts while preserving Vault, config, Grants, credentials, and all monotonic floors |
| A4 | Repair with old package or corrupt protected state | No floor rollback or silent overwrite; protected state locks or reports repair-required |
| A5 | Normal uninstall with empty Vault | Declared program/runtime state removed; required floors retained; zero undeclared residue |
| A6 | Normal uninstall with non-empty Vault and no verified Bundle | `uninstall-blocked` or Vault preserved in place; no invented secret/destination |
| A7 | Export verified Bundle then uninstall | Bundle remains only at Owner path; program/runtime removal completes; retained floors exact |
| A8 | Explicit purge and cancellation | Preflight enumerates loss; cancellation changes nothing; confirmed purge removes owned state best-effort and reports external-copy limitation |
| A9 | Development/H3/public state merge attempt | Denied; no cache, root, Vault, floor, Grant, or evidence crossing |

### B — Release decision and retrieval

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| B0 | Complete current threshold metadata and exact target from two distributors | Same `release-accepted` target and floors; distributor has no authority |
| B1 | Missing/duplicate/invalid/below-threshold root or Targets signature | `release-invalid`; no staging |
| B2 | Snapshot/timestamp or package source attempts to add executable target | `release-invalid`; only authorized Targets role can add bytes |
| B3 | Metadata version rollback, indefinite freeze, expired timestamp, fast-forward, or mix-and-match | Exact rollback/freeze/expiry/conflict class; no floor lowering |
| B4 | Wrong/missing target size/digest/platform/environment/build/SBOM/attestation policy | `release-invalid` or `release-incompatible`; no executable exposure |
| B5 | Oversized/count/depth/path/cache-confinement input | Bounded `release-invalid`/`resource-denied`; no write outside owned root |
| B6 | `private-only` source unavailable | `release-unavailable`; no direct contact |
| B7 | Explicit `direct-allowed` | Only precommitted finite direct sources contacted; disclosure limitation recorded |
| B8 | Complete/incomplete/tampered `offline-import` | Complete exact set accepted; incomplete/tampered set rejected without network fallback |
| B9 | Current/superseded/vulnerable/revoked build across protocol phases | Build and protocol machines evaluated independently; signed deadlines enforced |
| B10 | Consecutive root rotation signed by old and new thresholds | Each root is durably accepted in order; environment/root floors advance; no target becomes executable from root alone |
| B11 | Root version gap/reuse, one-sided threshold, expired/cross-environment root, or emergency root addition | `release-invalid`; installed trusted root and floors unchanged |
| B12 | Automatic startup/pre-work/pre-expiry Release Safety refresh | No forbidden installation/account/history fields; fresh lease may extend within signed bounds; failure never extends or triggers direct fallback |
| B13 | Ordinary protocol `required` before/after `90 days` and capacity/drain readiness | Before either conjunct: transition blocked; after both: transition accepted without changing build safety |
| B14 | Valid/invalid/expired `4-of-5` emergency transition | Only named safety emergency may shorten overlap/bypass capacity; cannot add executable/root; possible unavailability explicit; unratified expiry opens no unsafe work |

### C — Update transaction, interruption, and resources

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| C0 | Successful compatible update | Immutable stage, reserved rollback, bounded drain, atomic activate, self-test, commit; floors non-decreasing |
| C1 | Interruption before/after every durable transition | Restart recovers exact previous/current state or `transaction-invalid`; never guesses commit |
| C2 | Staging/hash/permission failure before stop-new-work | Current payload/work unchanged; complete staging cleanup |
| C3 | Disk/inode/entry pressure or missing rollback reserve | `resource-denied`; no stop-new-work and no partial stage residue |
| C4 | Drain reaches local or authenticated deadline | New work stopped; existing work closes honestly; no deadline extension or operation replay |
| C5 | Activated payload self-test fails | Safe authenticated rollback and `rolled-back`, or `repair-required` if rollback unsafe |
| C6 | Retained rollback payload revoked/incompatible/below floor | `rollback-refused`; no execution; local repair/export remains |
| C7 | Copy-on-write schema success/failure | Schema becomes current only at commit; failure leaves prior readable state and no Authority/floor rollback |
| C8 | Network unavailable during self-test | Does not alone mark payload bad; bounded unavailable or repair decision from declared offline/online checks |
| C9 | Forward and rollback start both fail | `repair-required`; no normal network work; repair/Authority export available |
| C10 | Repeated update/rollback/restart pressure | Retained payloads, journal, processes, disk, memory, handles, goroutines, timers, and logs remain within frozen bounds |
| C11 | Contributor update with active bounded role duties | New assignments stop, each role drains within earliest lease/deadline, old duty never revives, and rejoin requires fresh current safety/assignment/credentials or remains stopped/withdraws |

### D — Authority Vault and Recovery Bundle

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| D0 | Update/rollback/repair with active Vault | Exact Vault and authority watermarks preserved; runtime never reads root material |
| D1 | Bundle export and isolated test restore | Version/encryption/schema verified with no network or signing Interface; source Vault unchanged |
| D2 | Wrong secret, tamper, truncation, oversized, wrong environment/network/root | `bundle-invalid`; no partial material exposed or state activated |
| D3 | Restore then unavailable/stale/conflicting/forked reconciliation | `authority-locked`, export-only; no signature or runtime key restoration |
| D4 | Restore then current authenticated reconciliation | Strictly higher generation/revision before active signing; new runtime Instance Key; Grants separately reissued |
| D5 | Bundle contents and filenames | No Local Grants, Instance Keys, session/Route state, Application Data, or plaintext Name/Target disclosure |
| D6 | Crash/restart during export/restore/reconcile | One unambiguous source/temporary/final state; no plaintext temp residue or duplicated active authority |

### E — Application Principal, grant, and local channel

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| E0 | Authorized launcher-bound client/publisher/admin principal | Exact process tree/channel/grant/context/resource/deadline bound before active work |
| E1 | Ungranted hostile same-user sibling attaches | `principal-denied`; no other context/Service/diagnostic/Authority visibility |
| E2 | Bearer/capability copied or replayed off-channel/by sibling | Denied; capability alone grants nothing |
| E3 | PID reuse, process replacement, channel/pipe/socket substitution | Denied; no identity based on PID/path alone |
| E4 | Failed peer/token/impersonation/ownership query | Fail closed; broker privilege is not used for request |
| E5 | Application/broker/Endpoint restart | Old sessions/capabilities invalid; persistent policy requires fresh principal binding |
| E6 | Connection grant requests admin/custody or admin requests connection/custody | Denied by exact privilege lattice |
| E7 | Immediate revoke and finite drain-then-revoke | New work denied; descendants invalid; custody/admin immediate close; data follows selected finite policy |
| E8 | IPC/frame/session/process/handle/queue pressure and slow peer | Bounded `resource-denied`/backpressure; established unrelated work isolated; complete cleanup |
| E9 | Generic indistinguishable same-user attachment | Works only in coarse trust domain and reports unqualified sibling/network status |

### F — Network-isolated Application boundary

Each F cell runs from both controlled client and publisher Application trees and
includes direct code plus child, grandchild, delayed helper, shell/interpreter,
and inherited/duplicated-handle variants where the platform supports the action.

| Cell | Scenario | Expected runtime outcome |
|---|---|---|
| F0 | Scoped Ardents IPC and context storage | Declared local channel/storage works; no other Application context accessible |
| F1 | System/raw DNS and resolver helper | No query/packet/result outside declared Ardents IPC |
| F2 | IPv4/IPv6 TCP, UDP, QUIC, raw or local-network external connect | No packet or successful external socket |
| F3 | HTTP fetch/redirect/proxy, callback/webhook/SSRF, WebSocket, WebRTC/STUN | No direct path, packet, DNS, callback, or transparent fallback |
| F4 | Bind/listen on wildcard, loopback, host address plus external scan/connect | No ordinary reachable listener or ingress |
| F5 | Child/helper/process-tree breakaway, alternate user/session, delayed spawn | Every process remains contained/owned or launch is denied; no surviving helper |
| F6 | Inherited/duplicated/passed network handle or unrelated open descriptor | No network use; undeclared handles absent; attempted escape evidenced |
| F7 | Cross-Application IPC/storage/context access | Denied; no origin/cache/storage reuse across contexts |
| F8 | Grant revoke, broker crash, Application crash, Endpoint restart | Tree terminates within bound; policy/rules/profile/IPC/temp storage cleaned; old session unusable |
| F9 | Unsupported Application or unavailable isolation mechanism | `isolation-unsupported`; never generic or claim-bearing success |
| F10 | Generic profile runs same escape probes | Result remains `application-networking-unverified`; carrier result cannot be relabeled isolated |

### G — Platform pairing and maintained H3 use

Development uses a controlled client and publisher on each frozen image. R-054
freezes exact attempt counts. All four pairings run both Application Data
directions for the short Stage 7 matrix:

| Cell | Pairing | Required result |
|---|---|---|
| G0 | Ubuntu client -> Ubuntu publisher | exact Target and exact-name maintained path works under declared profile; platform evidence passes |
| G1 | Ubuntu client -> Windows publisher | same shared outcome and error taxonomy |
| G2 | Windows client -> Ubuntu publisher | same shared outcome and error taxonomy |
| G3 | Windows client -> Windows publisher | same shared outcome and error taxonomy |
| G4 | Update one endpoint during/after bounded work | explicit drain/connection terminal result; no Application-operation replay or weaker fallback |
| G5 | Restart and rebind after update/rollback | old local session invalid; new principal uses preserved policy and current security floors |
| G6 | Generic versus isolated report | generic limitation visible; isolated claim only when both endpoint trees pass |
| G7 | Ubuntu and Windows Contributor update during bounded role duty | Each host reports the same stop-new-work/drain/update/rejoin-or-withdraw outcome; no old assignment or handle survives |

Passing G cells is Stage 7 development evidence, not full cross-platform Route
Qualification. The complete post-cleanup qualification scope remains S9.6.

### H — Evidence integrity and cleanup

| Cell | Mutation or terminal scenario | Expected verifier |
|---|---|---|
| H0 | Complete valid reference artifact set | Recompute declared candidate result |
| H1 | Missing/duplicate/reordered/unknown-critical/wrong-version artifact or event | `invalid` |
| H2 | Source/host/platform/package/root/metadata/artifact mismatch | `invalid` |
| H3 | Manifest/evidence/verdict/private roots overlap or candidate writes verdict | `invalid` |
| H4 | Hash/path traversal/symlink/reparse/cross-volume/cross-cell substitution | `invalid` |
| H5 | Secret, plaintext Authority, reusable bearer, Name/Target, or private canary leaked to public evidence | `invalid` plus cleanup incident |
| H6 | Missing process/path/socket/pipe/handle/job/namespace/rule/package/service cleanup or undeclared residue | `invalid` when observation is unreliable; `fail` when valid evidence proves candidate residue |

## 4. Platform observation contract

For both OSes the authoritative observer records:

- process creation/parentage/tree membership, user/token/sandbox/job/namespace,
  executable digest, start/exit, and survivor inventory;
- file/registry/package/service/startup/rule/profile/IPC object creation,
  owner/ACL/mode, volume/filesystem identity, link/reparse status, size, and final
  cleanup;
- controlled interface packets/bytes, DNS observations, listeners and external
  scans, with candidate and harness traffic separated;
- CPU, resident/working-set memory, process/thread, handle/FD, storage, network,
  and deadline series for the complete charged tree; and
- monotonic transaction/principal/isolation events with wall time used only to
  correlate hosts.

Candidate-independent inability to observe a required fact makes the affected
cell `invalid`; it cannot be replaced with candidate self-report. Candidate
escape or observer interference caused by the candidate is `fail`.

## 5. Privacy statements

| Claim | Protected information | Adversary | Conditions | Measurement | Honest limitation |
|---|---|---|---|---|---|
| Update-source separation | Release authority from distributor and, in private-only mode, ordinary origin from a direct source | Malicious mirror/source | Current accepted metadata; exact mode; no fallback | Identical bytes decide identically; source contact inventory matches mode | Direct mode reveals origin/artifact/timing; H3 path and keys are project-controlled |
| Local principal separation | One Application's grant/context/diagnostics/Service authority | Same-user hostile sibling | Accepted launcher/OS binding and fresh session | E1-E8 denials and bounded views | Coarse generic apps may be one trust domain; OS/Owner compromise defeats it |
| Application network isolation | Endpoint location from Application-induced direct activity | Malicious peer/content/request or scanner | Qualified isolated profile on both complete endpoint trees | F1-F8 show no listener, packet, DNS, connect, escape, or storage crossover | Timing, behavior, content, credentials, intended peer plaintext, fingerprints, and OS compromise remain |

## 6. Falsification and stop conditions

Stage 7 evidence fails if valid evidence shows any of:

- distributor/package source authorizing bytes or threshold roles collapsing;
- accepted rollback/freeze/mix/wrong-platform/digest input;
- protocol/build state conflation or signed safety deadline extension;
- in-place active payload overwrite or guessed transaction commit;
- Authority/Vault/floor rollback, secret exposure, or Bundle-derived Grant/
  Instance Key;
- unsafe rollback, hidden network repair, or unavailable repair/export;
- non-empty Vault silently erased or backup secret/destination invented;
- PID/path/bearer/same-user identity accepted as a claim-bearing principal;
- failed OS identity/impersonation check continuing under broker privilege;
- grant privilege crossing or post-restart bearer survival;
- any isolated listener, packet, DNS query, external connection, child/helper
  escape, cross-context storage/IPC, or transparent generic fallback;
- platform-specific weaker outcome hidden behind a common success;
- project-controlled identities described as independent; or
- unbounded work, incomplete cleanup, or candidate-authored verdict.

## 7. Coding-start authority

The [readiness checklist](stage-7-readiness-checklist.md) sections A-D are the
sole normative entry gate. This evidence inventory neither authorizes a campaign
nor duplicates the decision states recorded there.
