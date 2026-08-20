---
id: R-053
title: Which custody and cryptographic profile protects Stage 7 Authority recovery?
status: decided
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

The exact retained O2 candidate is frozen in the
[Authority Custody specification](../../development/stage-7-authority-custody-spec.md)
and the consequential choice is summarized in the
[password-derived custody proposal](../../development/stage-7-password-derived-authority-custody-proposal.md).

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
- password/KDF choice and parameters measured on scheduled development surfaces
  and later on the weakest supported native qualification host,
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
  libraries after candidate enumeration; specifically Go 1.26
  [`cipher.NewGCMWithRandomNonce`](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce)
  and `golang.org/x/crypto/argon2` candidate `v0.52.0`.

### Experiment

The disposable
[R-053 logic prototype](../../../experiments/r-053-stage-7-authority-recovery/README.md)
freezes synthetic Authority vectors and secret commitments and exercises the
strict shared envelope plus locked-restore state model. It does not persist
bytes or qualify a cryptographic/platform implementation.

The remaining development-host campaign follows the
[Stage 7 development-host campaign specification](../../development/stage-7-host-campaign-spec.md)
and compares the fixed O2 candidate against O0;
O1 platform live-Vault Adapters are source-reviewed rejected alternatives, not
an implementation race. Measure KDF CPU/memory/latency,
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
`0.5–3 s` monotonic derivation on the weakest supported native qualification
host. The decoder MUST
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
- **Sourced fact:** Go 1.26 `NewGCMWithRandomNonce` generates and prepends a
  random 96-bit nonce and returns 28 bytes of nonce/tag overhead. It limits one
  key to fewer than `2^32` messages; the R-053 candidate is narrower and seals
  exactly one envelope per freshly salted derived key.
- **Sourced fact:** Windows DPAPI commonly binds protection to the same logon
  credentials and computer; machine scope permits any local user on that machine
  to decrypt and is not an acceptable default merely for convenience.
- **Sourced fact:** the 2026 Freedesktop Secret Service 0.2 document is still a
  draft and a desktop service may be absent on a headless Ubuntu host.
- **Inference:** O1 creates two platform unlock/recovery contracts without
  removing the portable Bundle password. It also makes the live Vault depend on
  a source OS account/machine or desktop service, while Stage 7 roots are used
  only through explicit custody operations.
- **Inference:** O2 is the smaller one-to-one-maintainable profile. Distinct
  authenticated `authority-vault`/`recovery-bundle` purposes and separately
  entered secrets preserve the live-versus-portable boundary without selecting
  two storage cryptosystems.
- **Measured prototype result:** the strict synthetic envelope was
  `1,121–1,122` bytes. Correct unlock/test verification passed; wrong password
  and a canonical ciphertext mutation both returned `bundle-unlock-failed`;
  wrong environment denied; a 512 MiB parameter rejected before KDF; restore
  remained locked until both generation and revision were strictly higher;
  activation created a fresh runtime-key commitment and no Grant.
- **Measured prototype limitation:** on Go `1.26.6 windows/amd64` with current
  `x/crypto v0.51.0`, single derivations were approximately `27–33 ms` at
  64 MiB, `66–69 ms` at 128 MiB, and `134–147 ms` at 256 MiB. This current
  Windows development host is not the weakest supported native host. It selects
  no qualification pass result.
- **Measured exact-dependency replay:** the full 64 MiB logic sequence also
  passed in a temporary module using proposed `x/crypto v0.52.0` at commit
  `a1c0d9929856c8aba2b31f079340f00578eda803` and checksum
  `h1:RMs7fP2rXdep0CftQlK8Uf+kibLm7qkCcradZWYz988=`. This closes version-skew for
  format/state coherence only; official vectors, scheduled 256 MiB resource
  evidence, and weakest-native-host qualification remain open.

## Options

- **O1:** portable password-derived AEAD Bundle plus platform-protected live
  Vault, both behind Authority Custody.
- **O2:** portable password-derived protection for both live Vault and Bundle.
- **O0:** choose none; keep Authority features locked/export-unavailable and stop
  Stage 7 rather than improvise cryptography.

## Recommendation

Advance O2 as the exact candidate: canonical
`ardents-authority-envelope-v1`, Argon2id v19 through `x/crypto v0.52.0` with
`256 MiB`, `t=3`, `p=4`, 16-byte salt and 32-byte key, then Go 1.26
AES-256-GCM random-nonce AEAD. Use password-derived encryption for both live
Vault records and Bundles, with distinct purposes and separate secrets. Bound a
v1 Vault to 1024 independently atomic records/1 GiB under one Vault password,
with no in-place password rotation. Do not
select DPAPI, Secret Service, automatic unlock, or a new OS dependency in Stage
7. Confidence is medium in format/state coherence and low-to-medium in the
fixed KDF/platform fit before weakest-native-host evidence.

## Disposition

- State: `decided`; the Product Owner accepted exact O2 and its password/loss
  semantics for Stage 7 development on 2026-08-20 under ADR-0021.
- The current Windows development measurement is below the precommitted
  `0.5–3 s` weakest-host band. The exact `256 MiB/t=3/p=4` candidate remains
  provisional until the weakest supported native host passes; failure reopens
  R-053 rather than changing the band after measurement.
- Scheduled Ubuntu-Docker/current-Windows vector/resource/interruption/cross-
  platform restore/cleanup evidence remains an S7.2 gate. The Product Owner
  accepted that weakest-native-host performance and unavailable durability
  cells are deferred, not passed; a falsifier reopens ADR-0021.
- R-044 remains separate and cannot be partially solved by this record.
