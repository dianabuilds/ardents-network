# H4-alpha-1 bounded release profile

Status: **recorded on 2026-08-27; not a signed release, published artifact, or
participant enrollment.** This is the one profile to which H4-8A evidence must
be bound before a bounded alpha can be accepted.

## Identity

| Field | Fixed value |
|---|---|
| Profile | `ardents-h4-alpha-1-v1` |
| Endpoint source revision | `70bf425eec937edcc22e8f0534db992aa2002a16` |
| Endpoint artifact | Ubuntu Portable `linux-amd64`, SHA-256 `33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1` |
| Control companion | `ardents-control-linux-amd64`, SHA-256 `d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c` |
| Release carrier | State-selected TCP/TLS v1 only; QUIC is not a fallback or alpha claim |
| Service journey | one exact Target Link, one loopback Browser Adapter origin, dynamic HTTP/1.1 workload |
| H4-4 relation | an alpha corpus may be a control input; Browser Entry and `.ard` resolution are not release-gating |

The artifact source revision is distinct from the local custody-tool revision.
The latter may prepare static inputs but is never bundled as the Endpoint and
does not change an already built artifact's identity.

## Public custody companions

The Product Owner authenticated the existing local encrypted seed record on
2026-08-27. Its public receipt is `ardents-release-custody-receipt-v1`; the
record ciphertext digest is
`0d14d5fcf9bc285e23d507a6382e9ff7100b2018acf182f6e65a885e52ec1738`.
This confirms custody of the selected public roots only. It is not a threshold,
independent-custody, signing, publication, or enrollment result.

| Role | Ed25519 public key (hex) |
|---|---|
| `tuf-top-level-1` | `ef8318aaf5064d18f9f668285eca91a3bdc5a429becead6ac4e32ae355b7ac25` |
| `tuf-top-level-2` | `6db5ba640bcf8e628ce69cae28a6230fb5c609627f6d3fd6a51fe26f7cf02b70` |
| `tuf-top-level-3` | `b17a732e98e5b5b6800a2c282a7e03b0c80ade2ad1d439c6cf576dd34c64d674` |
| `tuf-top-level-4` | `8ea3fe8a5f64af15f8e0e679a5f7cc40f434148010e9e097e89b6d279c963e32` |
| `tuf-top-level-5` | `81881a3915e823627faff648cf05617f56c53d921e6184a02e57285b7a9dd65d` |
| `alpha-disclosure` | `03975399cc9fc06e313590031273fe973e20ec5d6f3086f0bebe0bb5075a8c87` |
| `alpha-release-component` | `b697ae91f12613474427478054a54599294c18ed3f7603aed63588a764080597` |
| `alpha-network-component` | `cd8ef165b2f85fdd1a3efbb7c277822d963b0ef22dad19713ac1d739588c1a13` |
| `alpha-compatibility-component` | `ce64bedf7b14d461b09edfaf6009b9ef153cccc0c2cbc1a3d17f73ccca944eee` |
| `alpha-corpus-authority` | `2c4a74d89d24661be7fe77e249d69055d5d41d4e5e8e136caae20ecdc3df447e` |

## Required static inputs before signing

The fixed signer operation remains unimplemented until these public facts are
recorded and individually validated:

1. cohort, release identity/version, validity interval, emergency-stop rule,
   `alpha` environment, and selected Network identifier;
2. the complete accepted offline Network State epoch, authority public keys,
   threshold, profile, materialization inputs, and topology disclosure;
3. the exact H3 TUF target descriptor for the Endpoint artifact, including two
   distinct builder attestations, source/build/dependency/SBOM identities,
   qualification/build/protocol state, and safety dates; and
4. an all-or-nothing external output root and verifier preflight receipt.

ADR-0052 selects the future fixed local `BuildAlphaInputs` operation that may
consume these facts. No value in this profile permits arbitrary bytes, a
substitute authority, an online signer, an upload, or participant contact.

## Qualification coherence

The exact profile bytes now passed A1 source integrity; the A2 Portable and A3
replacement/rollback Ubuntu user-session qualifications; constrained VPS Docker
A5/A6/A7; the A8 Windows-to-VPS tracer; and A9 Firefox 154.0.1 browser
observation. Their retained logs and available exit receipts are named in the
readiness matrix. These results remain bounded test/qualification evidence, not
artifact provenance, enrollment, participant, independent-operation, or
browser-isolation claims.

## Claims and limitations

If every required H4-8A row later passes for this exact profile, the strongest
claim is a bounded project-operated alpha journey. It remains non-public and
does not claim independent control or operators, capacity, availability,
censorship resistance, application-level privacy, public DNS/HTTPS, Namespace,
or Public Beta readiness.

## Ownership

This profile is owned by H4-8A. Its artifact/enrollment transition is H4-1;
dynamic browser transport evidence is H4-3B; the optional corpus remains H4-4;
and the independently rooted release/network/compatibility statements are
H4-6A. See the [readiness matrix](08a-alpha-1-readiness-matrix.md) and the
[closed-alpha ceremony](../../development/closed-alpha-release-ceremony.md).
