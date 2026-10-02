# Independent issuer key material

This bounded offline owner prepares immutable RSA keys and an unsigned local
inventory. It does not appoint an issuer, authenticate State, sign ARDCIP01,
issue tokens or provide network admission. Admission and Hosting remain separate.

## Interface and identity

`Initialize(ctx, root, Binding) error` generates before creating a fresh root.
`Open(ctx, root, Binding) (Store, error)` holds an exclusive Linux lease until
`Close`. Store copies share lifecycle; `Inventory` returns a protective copy.
No private-key API exists. Binding fixes Network, issuer, public Node signer and
one to six aligned UTC hours. Future preparation and expired inspection are valid.
Each hour/class 1–3 has a distinct validated two-prime RSA2048/E65537 key.

The private encoding is versioned binary: magic, fixed binding, cohort count,
then sorted window/class/DER-length/PKCS#1 DER. Reject noncanonical DER, invalid
keys, duplicate public keys, missing cohorts and unsigned-time overflow.
Public inventory canonical bytes are a separate domain magic, fixed binding,
count, then sorted window/class/346-byte canonical RSA-PSS SPKI. SHA-256 of
these bytes identifies the inventory. The exported JSON has a local schema,
explicit binding, entries and digest; it is never a signed profile.

## Storage and failure

Linux root is owner-only 0700; every file is owner-only 0600 with one hard link.
Reject symlinks, changed file/root identities, unknown files and foreign formats.
Exact retained set: issuer.pin, issuer.keys, issuer.lock. Pin contains the
ardents-issuer-key-material-v1 marker, binding, material and inventory digests.
Material is bounded at 64 KiB. Initialize uses an exclusive issuer.pending,
write/sync/close, rename, readback and directory sync; publish pin last and sync
root and parent. Any failure after mkdir is uncertain, with no recovery/removal.
Open rejects pending/partial roots, locks nonblocking, validates every key and
pin and performs file/root/parent durability barriers before acknowledgement.
Close is shared and idempotent. Context cancellation is checked before changes
and between standard RSA generations; a single generation is not interruptible.
Unsupported platforms refuse before effects. No rollback/power-loss claim.

CLI configs have exact root/binding/inventory_file fields, bounded strict JSON,
lowercase hex and whole-second UTC times. Export uses a separate existing owned
directory: exclusive new regular file, or byte-identical retry with sync. A
different existing file refuses; export errors never reset the key root. Only
finite operation/phase/outcome and duration reach diagnostics and OTel.

## Transfer and acceptance map

| Rule | Current dev provenance | Independent acceptance |
| --- | --- | --- |
| RSA generation/PKCS#1 validation | route/credential/closed_issuer_root.go | real 1h/6h generation, malformed DER/parameters and key reuse |
| Canonical SPKI | same owner encodeClosedIssuerSPKI | exact ASN.1 bytes and new Admission command compatibility |
| Sorted finite cohorts/binding | same owner closedIssuerPublicKeys/material codec | missing/duplicate cohorts, binding/digest/time overflow |
| Owned files and exclusive lease | credential issuer-root platform/storage | actual permissions/link/substitution/busy/crash/reopen |
| Durable immutable identity | existing issuer-root flush/reopen | write/sync/rename/close faults, pending and partial roots |
| Independent root format | approved slice contract | reciprocal legacy refusal without effects |
| Bounded export and telemetry | successor command composition | real CLI exact retry/conflict, OTLP payload and failed collector |

Selected execution: host quick/full checks and pinned Go1.27.1 Linux Docker
behavior/race/process profile with real files and compiled command, proc/sys/lo
available for existing command tests. Fault controls are private test inputs;
no mock supplies missing production behavior. Evidence and source identities
stay outside the repository. Commit acceptance requires the complete CLI cycle
and unchanged inventory after restart; no qualification of the network follows.
