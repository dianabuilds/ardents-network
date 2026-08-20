---
id: R-053
title: Which custody and cryptographic profile protects Stage 7 Authority recovery?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-053 — Stage 7 Authority Vault and Recovery Bundle

## Decision this unlocks

Select the maintained cryptographic/profile and platform custody Adapters for
Authority Vault protection, explicit encrypted Recovery Bundle export, isolated
test restore, authenticated reconciliation, `authority locked`, export-only,
and destructive-purge reporting in S7.2–S7.4/S7.7.

## Current contract

R-024, R-048, the operating model, and lifecycle specification fix Bundle
contents and exclusions, owner-chosen secret/destination, no cloud/help-desk
recovery, no signing during test restore, strict post-restore monotonic advance,
new runtime Instance Key, separately reissued Local Grants, non-empty-Vault
uninstall blocking/preservation, and honest best-effort deletion limits.

Stage 6 Recovery Authority threshold cryptography is a separate R-044 decision.
R-053 protects local/offline backup bytes; it MUST NOT select or reuse the
Namespace Recovery Authority mechanism by inertia.

## Hypotheses

- **H1:** one versioned portable Bundle format using a reviewed memory-hard KDF
  and standard-library/reviewed AEAD, plus platform-protected live Vault
  Adapters, meets both hosts without merging platform and portable trust.
- **H2:** a portable password-protected Vault and Bundle is simpler and safer
  than platform live-Vault storage, accepting explicit owner interaction.
- **H0:** available implementations cannot meet memory, secret-handling,
  portability, tamper, recovery, and one-to-one maintenance requirements.

## Evaluation criteria

- no implemented cryptographic primitive; exact maintained library and vectors;
- version/domain/environment/network/root binding and canonical bounded format;
- password/KDF choice and parameters measured on frozen weakest/normal hosts,
  resisting trivial offline guessing without exceeding resource budgets;
- unique salt/nonce, authenticated encryption, associated metadata, tamper/
  truncation/wrong-secret uniform failure, and nonce/key lifecycle;
- platform live-Vault protection semantics, account/machine/backup portability,
  privilege, prompts, headless behavior, lock/unlock, dependency, and failure;
- zero Grants, Instance Keys, sessions, Route state, Application Data, or
  plaintext Name/Target filenames;
- atomic export/test/restore/reconcile, restart recovery, temp/memory cleanup,
  and no duplicated active authority;
- Owner-selected destination with no automatic network/cloud write; and
- license, maintenance, audit/advisory/source identity, Go 1.26 support, removal,
  and explicit best-effort deletion limitation.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [RFC 9106 Argon2](https://www.rfc-editor.org/rfc/rfc9106.html), including
  Argon2id vectors and parameter recommendations;
- [NIST SP 800-38D AES-GCM](https://csrc.nist.gov/pubs/sp/800/38/d/final), noting
  its pending revision and nonce/misuse implications for profile design;
- Microsoft [CryptProtectData / DPAPI](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata),
  including user-vs-machine binding and current prompt deprecation;
- Freedesktop [Secret Service 0.2 draft](https://specifications.freedesktop.org/secret-service/latest/),
  which is candidate context rather than an assumed Ubuntu foundation; and
- Go standard-library AEAD/random/hash source plus exact maintained KDF/platform
  libraries after candidate enumeration.

### Experiment

Create `experiments/r-053-stage-7-authority-recovery/`. Freeze synthetic
Authority vectors and secret commitments. Compare at least two credible portable
profiles and live-Vault Adapters on both hosts. Measure KDF CPU/memory/latency,
bundle size, wrong-secret/tamper timing classes, interruption at every export/
restore/reconcile transition, restart, unavailable/stale/conflicting/forked
state, strictly higher success, temp/memory/file cleanup, uninstall/purge, and
cross-platform recovery where required. Raw reusable secrets stay outside Git
and public evidence.

### Failure scenarios

Nonce reuse; unauthenticated header; environment/root substitution; parameter
DoS; low-memory crash; OS account/machine loss; deprecated prompt dependence;
headless Ubuntu absence; plaintext temp/swap/diagnostic; rollback restore signs;
runtime key/Grant derived; partial export overwrites good Bundle; invented
destination/secret; two active restored authorities; and false secure-deletion
claim.

## Falsification criteria

Freeze synthetic Authority vectors, the weakest-host identity, and resource
bands before measuring candidates. H1/H2 is falsified if any wrong-secret,
tamper, truncation, nonce-reuse, cross-environment, rollback, interruption,
duplicate-active-authority, plaintext-temp, forbidden-content, or reconciliation
case succeeds once; if portable restore requires the source OS account/machine;
or if implementation needs first-party cryptography, cgo/unsafe, an incompatible
license, or an unpatched called high/critical advisory.

An acceptable password-derived profile must fit `64–256 MiB` KDF memory and
`0.5–3 s` monotonic derivation on the frozen weakest host. The decoder MUST
reject before allocation any encoded request above `512 MiB` memory, `10` passes,
`4` lanes, a `64 KiB` header, or a `16 MiB` total Bundle. Normal export/restore
may use at most `512 MiB` peak RSS and `10 s` outside platform user-consent UI.
The exact selected parameters are chosen only inside these precommitted bands;
if no candidate meets every security, portability, and resource conjunct, select
O0 rather than tune the band after results.

## Findings

- **Sourced fact:** RFC 9106 defines Argon2id and test vectors but its parameter
  choices have large memory costs that must be measured on the supported client
  profile rather than copied blindly.
- **Sourced fact:** AES-GCM is an authenticated-encryption mode, but correct
  nonce/profile handling remains the application responsibility and NIST is
  revising SP 800-38D.
- **Sourced fact:** Windows DPAPI commonly binds protection to the same logon
  credentials and computer; machine scope permits any local user on that machine
  to decrypt and is not an acceptable default merely for convenience.
- **Sourced fact:** the 2026 Freedesktop Secret Service 0.2 document is still a
  draft and a desktop service may be absent on a headless Ubuntu host.
- **Inference:** portable Bundle encryption and live Vault protection are
  separate seams and may choose different Adapters while sharing authority-state
  semantics.

## Options

- **O1:** portable password-derived AEAD Bundle plus platform-protected live
  Vault, both behind Authority Custody.
- **O2:** portable password-derived protection for both live Vault and Bundle.
- **O0:** choose none; keep Authority features locked/export-unavailable and stop
  Stage 7 rather than improvise cryptography.

## Recommendation

Measure O1 and O2 with exact maintained implementations. Prefer O1 only if both
platform live-Vault Adapters work in declared desktop/headless conditions and do
not weaken portability or operator recovery. Create an ADR for the accepted
cryptographic/custody profile. Confidence: low before experiment.

## Disposition

- State: `open`; no KDF, AEAD, Bundle format, dependency, platform vault, or
  numeric parameter selected.
- Required before S7.2 custody-preservation evidence and Stage 7 coding start.
- R-044 remains separate and cannot be partially solved by this record.
