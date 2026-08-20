# Stage 7 Authority Custody specification

Status: **accepted for Stage 7 development on 2026-08-20 under ADR-0021.** The
R-053 scheduled development-host KDF/resource/interrupt evidence, weakest-native-
host qualification gate, and current dependency/advisory review remain required
for the owning implementation slice.

This document freezes the exact accepted O2 profile to implement and falsify. It protects local Vault
records and portable Recovery Bundles with one password-derived envelope while
keeping their purposes, state transitions, and destinations distinct. It does
not select Namespace Recovery Authority cryptography, cloud recovery, a
password manager, OS account recovery, or secure deletion.

## 1. Selected candidate and trust boundary

The candidate is `ardents-authority-envelope-v1`:

- Argon2id v1.3 through `golang.org/x/crypto/argon2` candidate `v0.52.0`;
- fixed qualification candidate `memory=262144 KiB`, `passes=3`, `lanes=4`,
  `salt=16 bytes`, and `output=32 bytes`;
- AES-256-GCM through Go 1.26
  `crypto/aes` plus `cipher.NewGCMWithRandomNonce`;
- one fresh salt and one encryption per derived key; the Go AEAD generates and
  prepends one random 96-bit nonce and appends the 128-bit tag;
- strict byte-canonical JSON envelopes with Raw URL Base64 without padding for
  random/secret bytes; and
- password-derived protection for both the live Authority Vault and portable
  Recovery Bundle. DPAPI and Secret Service are not selected in Stage 7.

The Authority Custody Module alone may unlock, create, export, test-restore,
reconcile, or sign with root material. Connection, Service Administration,
Application Broker, Install Lifecycle, Update Transaction, runtime Endpoint,
and evidence writers never receive the password, derived key, plaintext root,
or signing Interface.

The candidate accepts explicit owner interaction rather than storing an
automatic platform unlock key. This gives the same headless/desktop semantics
on Ubuntu and Windows, keeps Recovery portable across OS account/machine loss,
and avoids a second cgo/unsafe or D-Bus secure-storage dependency. It also means
the Vault is locked after every process restart and unattended custody/signing
is unavailable in Stage 7.

## 2. Secret input and password rules

The password is an exact opaque byte sequence of `16..1024` bytes. Ardents does
not trim, case-fold, normalize Unicode, recover, escrow, log, persist, cache
across a custody operation, place it in argv/environment/configuration/evidence,
or invent it. Interactive input is confirmed through a terminal without echo;
headless automation may use only a pre-opened dedicated secret descriptor or an
in-process callback owned by the invoking custody boundary. A pathname, command
substitution, environment variable, or stdin shared with Application Data is
not a password channel.

Creating a Vault password and exporting a Bundle each require double entry.
Bundle export requires a newly supplied password distinct byte-for-byte from
the currently unlocked Vault password; there is no silent reuse. Losing either
password makes its protected bytes unrecoverable. Minimum length does not prove
entropy, and local unlock throttling cannot prevent offline guesses against a
copied envelope; the UI must state both limitations.

The implementation overwrites owned password, derived-key, salt-copy, nonce-
copy, plaintext, and root buffers immediately after the custody step and keeps
them out of errors/diagnostics. Go heap copies, compiler/runtime behavior, swap,
hibernation, crash dumps, privileged debuggers, and endpoint compromise remain
honest limitations. Stage 7 selects no `unsafe`, cgo, `mlock`, or false
guaranteed-zeroization claim.

## 3. Exact outer envelope

The UTF-8 envelope has no BOM, insignificant whitespace, trailing newline,
unknown field, duplicate field, alternate number/string form, padding, or
trailing byte. The decoder reads at most `16 MiB`, decodes into a fixed struct
with unknown fields forbidden, re-encodes, and requires byte equality before
KDF work.

Fields occur in this exact order:

```json
{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"recovery-bundle","kdf":{"name":"argon2id","version":19,"memory_kib":262144,"passes":3,"lanes":4,"salt":"<22-char-base64url>"},"aead":"aes-256-gcm-random-nonce","ciphertext":"<base64url>"}
```

Exact semantics:

| Field | Rule |
|---|---|
| `profile` | exactly `ardents-authority-envelope-v1` |
| `schema_version` | integer `1` |
| `purpose` | exactly `authority-vault` or `recovery-bundle` |
| `kdf.name/version` | exactly `argon2id` / `19` |
| `memory_kib/passes/lanes` | exactly `262144` / `3` / `4` |
| `salt` | Raw URL Base64 of exactly 16 fresh random bytes |
| `aead` | exactly `aes-256-gcm-random-nonce` |
| `ciphertext` | Raw URL Base64 of nonce-prefixed AES-GCM output; decoded size `28..8 MiB+28` |

The canonical associated data is the exact outer object through `aead`, with
the `ciphertext` field omitted and all other bytes/order unchanged. Therefore
purpose, profile, schema, KDF identity/parameters/salt, and AEAD identity are
authenticated. A parameter is validated against the fixed profile before
Argon2 allocation; no encoded request may select `512 MiB`, `10` passes, or any
other merely upper-bounded value.

Every write generates a new salt and derives a new AES key. That key seals one
message exactly once and is discarded. Import/open derives a fresh local copy
for authentication and discards it after one open. Ardents never supplies a
nonce, exposes a key, accepts nonce-size/tag-size variants, or re-encrypts under
an old salt/key.

## 4. Exact encrypted authority state

The authenticated plaintext is canonical UTF-8 JSON no larger than `8 MiB`.
Its exact field order is:

```json
{"profile":"ardents-authority-state-v1","schema_version":1,"purpose":"recovery-bundle","environment":"<sha256>","network":"<sha256>","root":"<sha256>","authority":{"kind":"service","id_commitment":"<sha256>","root_material":"<base64url>","generation":3,"revision":7,"watermarks":[{"domain":"credential-generation","value":3}]}}
```

Rules:

- `purpose` exactly equals the authenticated outer purpose;
- environment, network, root, and authority identity are lowercase 64-character
  SHA-256 commitments to already canonical identities; plaintext Service Names
  and Targets are absent;
- `kind` is exactly `service` or `name`;
- root material is Raw URL Base64 of `1..65536` bytes;
- generation and revision are unsigned 64-bit decimal integers;
- `watermarks` has `1..32` entries, sorted strictly by unique ASCII domain;
  each domain is `1..64` bytes and each value is unsigned 64-bit;
- one v1 envelope contains exactly one Authority. The Authority Vault is a
  bounded set of at most `1024` independently atomic envelope records totaling
  at most `1 GiB`, not one all-or-nothing database; and
- no field can encode a Local Grant, runtime Instance Key, session/bearer,
  Route state, Application Data, plaintext Name/Target label, package/update
  key, recovery password, or cloud destination.

Bundle filenames are Owner-chosen; Vault filenames use the exact random form
below. Neither may contain Name, Target, authority kind, identity commitment,
generation, or revision. Protected-state indexes contain only the minimum
opaque record ID and state needed to find an envelope; they confer no signing
power.

A v1 Vault record ID is exactly 16 fresh random bytes rendered as 32 lower-case
hex characters; its file is `record-<id>.json`. The protected owner of an
Authority stores that opaque ID and supplies the expected environment/network/
root/authority commitments at open. Directory enumeration never selects an
Authority by position or filename meaning. Swapping/copying record files cannot
activate the wrong Authority because the decrypted commitments and lifecycle
state must match the expected protected owner before signing.

## 5. Admission and failure order

The decoder executes in this order:

1. total byte bound and exact canonical outer JSON;
2. supported profile/schema/purpose and exact KDF/AEAD parameters;
3. salt/ciphertext alphabet and decoded length bounds;
4. password-channel and password-length validation;
5. Argon2id derivation and AES-GCM authentication;
6. exact canonical inner JSON and field/count/length/order bounds;
7. environment/network/root/authority semantic binding; and
8. lifecycle-state and monotonic reconciliation checks.

Unknown profile/version/algorithm/parameters return `bundle-unsupported` or
`vault-unsupported` before KDF. Malformed, oversized, forbidden, or semantically
invalid input returns `bundle-invalid`/`vault-invalid`. Wrong password and any
canonical authenticated-data/ciphertext/tag mutation return the same bounded
`bundle-unlock-failed`/`vault-unlock-failed`, expose no partial plaintext, and do
not state which input was wrong. A successfully authenticated wrong environment,
network, root, or authority remains invalid/locked; no material is activated.

At most one KDF runs per explicit attempt, at most three consecutive attempts
run per custody invocation, and a local exponential delay applies thereafter.
This bound protects local resources and does not claim to slow an offline
attacker.

## 6. Vault persistence and update separation

Each Vault record is encrypted at rest and locked by default. The Owner unlocks
only for one bounded custody operation. Runtime work continues with already
issued bounded Credentials/Instance Keys; it never unlocks the root.

One v1 Vault has one Owner password used for every record; every record still
has a fresh independent salt/key/nonce. Stage 7 does not select in-place Vault
password rotation. A future rotation must use a whole-Vault successor
transaction with interruption/recovery evidence and a consequential accepted
decision; it cannot partly re-encrypt records under mixed passwords.

Vault records, authority signing watermarks, and an independently persisted
non-decreasing local authority floor live outside package payloads, versioned
activation, rollback payloads, update staging, disposable cache, and ordinary
uninstall ownership. Every Vault mutation uses an encrypted-only platform
transaction:

1. create same-directory/same-volume private temporary output;
2. construct plaintext only in owned memory;
3. seal once with a fresh salt/key and erase owned plaintext/key buffers;
4. write ciphertext, flush file, close, atomically replace, flush parent, and
   reopen/verify ciphertext and ACL/mode;
5. advance the separate authority floor only after the new record is durable;
   and
6. recover after interruption to exactly old or new record/floor, otherwise
   `authority-locked`/`repair-required` without signing.

Ubuntu requires owner `0600` files and `0700` directories. Windows requires an
explicit current-owner plus required system/recovery DACL, no inherited broad
ACE, and durable same-volume replacement. R-050 owns the final platform file
operations. No plaintext temporary file exists.

The separate floor detects an old record under normal update/recovery behavior;
authenticated network reconciliation detects stale Bundles. Rolling back every
protected file with a filesystem/hardware snapshot, malicious Owner/OS, or
storage compromise remains outside the claim and is stated explicitly.

## 7. Export, test restore, and reconciliation

Bundle export is explicit and receives an Owner-selected local destination and
new password. Ardents never invents a filename/path, writes to network/cloud,
opens a provider, emails/uploads a Bundle, or records the destination in public
evidence. Existing destination replacement requires separate confirmation;
failure leaves the prior good Bundle byte-identical.

Export builds one `recovery-bundle` envelope in memory, writes only encrypted
temporary/final bytes, reopens the final path, and performs an isolated test
restore using the newly re-entered Bundle password. Test restore has no network,
signing Interface, Vault-write Interface, Grant store, runtime-key store, or
active Authority slot. Success records only envelope digest, profile, opaque
authority commitment, and test result; never password/root material.

Restore never overwrites an active Vault record. It creates a separate encrypted
quarantine record with state `authority-locked` and export-only. To activate,
the custody boundary authenticates current environment/network/root/Namespace
state and computes a successor strictly greater than **every** applicable
Bundle, local-floor, and current-network generation/revision/watermark. It
durably writes the successor and floor before enabling the first signature.

Unavailable, stale, equal, conflicting, forked, wrong-environment, wrong-root,
or non-strict state remains `authority-locked` and export-only. Activation
creates a new runtime Instance Key through its separate lifecycle. Local Grants
remain absent and require separate explicit local-policy reissue. No Bundle bit
derives either object. Only one Authority record may become active; quarantine
and prior records cannot sign concurrently.

## 8. Resources, cleanup, and campaign gate

Candidate bounds are:

- exact KDF `256 MiB`, `t=3`, `p=4`; weakest-supported-native-host derivation must be
  `0.5..3 s` monotonic;
- total custody peak RSS at most `512 MiB`, one KDF at a time, and `10 s` outside
  explicit user-consent input;
- envelope `16 MiB`, plaintext `8 MiB`, non-ciphertext header `64 KiB`, password
  `1024` bytes, root `64 KiB`, 32 watermarks, 1024 Vault records/`1 GiB` total,
  and three attempts/invocation;
- encrypted temporary/final files only, with exact old/new/interrupted state;
  and
- owned memory/file/handle/temp cleanup within `5 s` after terminal result.

The disposable R-053 prototype passed canonical envelope authentication,
uniform wrong-secret/authenticated-tamper failure, pre-KDF parameter rejection,
wrong-environment denial, locked restore, stale denial, and strictly-higher
activation using synthetic state. On the current Windows development host its
256 MiB derivation took only about `134..147 ms`, so it is not platform
qualification. If the weakest supported native host is also below `0.5 s`, or
if either host exceeds time/RSS bounds, the fixed candidate is falsified and
R-053 reopens; the band is not changed after measurement.

The scheduled Ubuntu-Docker/current-Windows campaign must additionally pass
official Argon2id vectors, Go AEAD vectors, current reachable-advisory/license/source review,
wrong password, canonical AAD/ciphertext/tag mutations, truncation, every bound,
interruptions at each persistence transition, update/rollback/uninstall/purge,
cross-platform Bundle restore, unavailable/stale/conflicting/forked/current
reconciliation, fresh runtime key, absent Grants, plaintext-temp scans, memory/
swap/dump limitations, and external-copy/secure-deletion reporting.
Weakest-native-host performance and unavailable native/durability cells remain
explicit qualification deferrals rather than silent passes.

One successful forbidden case, plaintext artifact, rollback activation,
duplicate active Authority, restored Grant/runtime key, observer miss, or
unbounded allocation is `fail`/`invalid`, reopens ADR-0021, and blocks the
Stage 7 handoff. Maintained implementation is limited to the accepted profile,
owning slice, dependency record, package map, and repository risk gates.
