# H4-alpha-1 bounded release profile

Status: **historical bounded functional-alpha profile accepted on 2026-08-28.**
Its exact Ubuntu artifact is signed, published as an immutable prerelease, and
was exercised through the Product Owner's own authenticated enrollment. It is
not an independent-participant/Public Beta profile or the post-refactor
qualification candidate.

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
This receipt confirms custody of the selected public roots. The corresponding
encrypted record was later consumed by the fixed operation for the exact RC
described below; the receipt by itself is not a threshold,
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

## Network State custody companion

The Product Owner created the separate ADR-0053 functional-alpha State genesis
locally on 2026-08-27. Its encrypted record SHA-256 is
`f37a6f066dc813159fbfc58c98ffe38ec7378c81f5e104af3b7679c85869487c`.
The Network identifier is
`7dedac753091495fb6cbf69ed229a0ee1756b285ee0ae68bf257200ce6585ea2`;
the sole Epoch authority public key is
`959dd386634dc2d62c4b84f6d027a0f55faee5d3f8fa3d949580b6e8db6d96f0`;
and genesis digest is
`86852e7cef6fc3db842e4415721e2d9de8bb926a700900252dace11fb3ca634e`.
It uses threshold `1`, profile `ardents-interactive-route-v1`, and is valid
from `2026-08-27T20:57:39Z` until `2026-09-26T20:57:39Z`.

Its committed candidate view is deliberately empty and discloses
`empty-no-persistent-node`. This is the actual initial topology, not a
capacity or availability failure hidden behind test identities. H4-2's
temporary two-host State remains separately labelled qualification evidence;
it is not a persistent participant network or a release authority.

## Signed static inputs and immutable publication

The fixed ADR-0052 operation consumed the following recorded public facts on
2026-08-28 and completed its maintained verifier preflight:

1. cohort, release identity/version, validity interval, emergency-stop rule,
   `alpha` environment, and selected Network identifier;
2. the complete accepted offline Network State epoch, authority public keys,
   threshold, profile, materialization inputs, and topology disclosure
   (**recorded above; verifier-accepted empty topology under ADR-0053**);
3. the exact H3 TUF target descriptor for the Endpoint artifact, including two
   distinct builder attestations, source/build/dependency/SBOM identities,
   qualification/build/protocol state, and safety dates; and
4. an all-or-nothing external output root and verifier preflight receipt.

The public request SHA-256 is
`c397544a5d9c8adc811ed5f34b15978b0c315b60ea7692b115fdffdbdbacc36d`;
the verifier-accepted alpha-input receipt SHA-256 is
`f143400b7c083971995232d887b6760a7711f75ff3b90a32d607bdac92e816e1`;
and the resulting static-set digest is
`078553812ae691fc120bcfbf1d003bcf7096abdbe71796845d2f567b01c472e3`.
The archive was assembled twice with `SOURCE_DATE_EPOCH=1787864259` and was
byte-identical. Its SHA-256 is
`e7ff0b26257978fd14bc3583c5de7d36eb7626bac7b43586bcb9442c53f7dba7`;
the unpacked `SHA256SUMS` digest and Alpha Enrollment Pin is
`8ed0fd25c60a6988fcc8938baf86547c7c646744f57fb0c39186f184d13afefd`.

The immutable prerelease is
[`h4-alpha-1-rc-1`](https://github.com/dianabuilds/ardents-network/releases/tag/h4-alpha-1-rc-1),
published at `2026-08-28T00:10:15Z`. Its tag resolves to source revision
`70bf425eec937edcc22e8f0534db992aa2002a16`; GitHub release attestation and
local-asset verification both passed. The public release receipt SHA-256 is
`b9f0940d505ecfd19755ace159ce3a9f47931da4d9098e94b9a9a633a2ccce28`.

The two exact project-controlled build observations are labelled
`product-owner-windows-build-a` and `product-owner-windows-build-b`; they
produced byte-identical Endpoint and control artifacts and make no independent
builder claim. The retained canonical build-input evidence has SHA-256
`845e27cc24dc06617911c69ded532a0d8d07804b41502c9112f8553c7748b074`.
The linked-module/build-setting inventory produced by `go version -m` for both
artifact classes has SHA-256
`8d41ba4dafee520540d8b6208e57291acc8d6c5571c88f1b7b3f4ce7d66fd0be`.
The source `go.mod` and `go.sum` digests are respectively
`af58c2883fa27ceebe8e0efb1c88fc6f59f49fd8848b46d471b1fb481b023fe5`
and
`1dbf939c239e7ebec2d4b6f15d60cef05f0176c8e8a15f085388c003a8bf0850`.
Both observations used Go `1.26.6`, `linux/amd64`, `GOAMD64=v1`,
`CGO_ENABLED=0`, and `-trimpath`.

ADR-0052's fixed local `BuildAlphaInputs` operation consumed these facts. No
value in this profile permits arbitrary bytes, a substitute authority, an
online signer, or an ambient upload/contact capability. The maintained
operation enforced this profile's source revision, Endpoint/control digests,
selected encrypted-envelope digest, validity bounds, empty Network topology,
and verifier preflight before publishing the static output directory.

## Qualification coherence

The exact profile bytes now passed A1 source integrity; the A2 Portable and A3
replacement/rollback Ubuntu user-session qualifications; constrained VPS Docker
A5/A6/A7; the A8 Windows-to-VPS tracer; A9 Firefox 154.0.1 browser observation;
and the completed A10 local plus Windows-to-Ubuntu Carrier evidence. The A10 multihost runner
requires the exact candidate path and expected SHA-256 instead of silently
rebuilding the Endpoint from the checkout. Its product-Route cell ran two
instances of that exact byte while State-run product Initiator and Responder
processes opened their authenticated TCP/TLS v1 legs to the product Rendezvous.
The matching 8 MiB Application bytes, clean Endpoint terminal results, one
Route generation, zero recovery, and TCP/TLS-only READY events close the
functional-alpha Carrier gate without adding Browser Entry or public naming.
The same immutable bundle then passed the external Pin/inventory check,
Endpoint enrollment check, non-lingering first start, retained-state
stop/restart, final cleanup, and H4-6A fresh/cached/fresh observation. The
retained logs, failed diagnostic attempts, and exact digests are named in the
readiness matrix. These results establish bounded artifact provenance and a
Product Owner enrollment walkthrough; they do not establish an independent
participant, capacity, availability, independent operation, or browser
isolation.

## Claims and limitations

A1-A10 now pass for this exact immutable profile. The strongest claim is a
bounded project-operated functional-alpha journey. It remains a closed
prerelease and does not claim independent control, external participants or
operators, capacity, availability, censorship resistance, application-level
privacy, public DNS/HTTPS, Namespace, or Public Beta readiness. Its historical
A11 soak/fault and A12 closure do not transfer to a post-refactor candidate.

## Ownership

This profile is owned by H4-8A. Its artifact/enrollment transition is H4-1;
dynamic browser transport evidence is H4-3B; the optional corpus remains H4-4;
and the independently rooted release/network/compatibility statements are
H4-6A. See the [readiness matrix](08a-alpha-1-readiness-matrix.md) and the
historical R-119/R-120/R-121 custody records.
