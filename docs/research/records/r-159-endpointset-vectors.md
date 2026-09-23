# R-159 signed EndpointSet decision vectors

Status: **research fixtures**, 2026-09-23. These bytes specify a proposed ARNR schema-3 decision; current State intentionally rejects schema 3. They are not evidence of a successful Epoch/ClosedProfile acceptance or destination dial. See [R-159](r-159-endpointset-profile.md).

## Common deterministic input

Generate an Ed25519 key from the 32-byte seed 00 01 ... 1f using Go's crypto/ed25519.NewKeyFromSeed. The public key is:

    03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8

Use Network = 0x11 repeated 32 times, Node = 0x22 repeated 32 times, generation = 1, notBefore = Unix 0, notAfter = Unix 2147483647, existing length-u8 family text = 01 66 ("f"), capability = 02, capacity = 1. The proposed EndpointSet is one TCP/TLS-v2 IPv4 tuple (192.0.2.1, port 1); this documentation address does not authorize a dial.

    EndpointSet = 82018184010444c000020101
    endpointSetLength = 000c

P1's exact unsigned record is 144 bytes, then the 64-byte signature yields 208 bytes:

    41524e52031111111111111111111111111111111111111111111111111111111111111111222222222222222222222222222222222222222222222222222222222222222200000000000000010000000000000000000000007fffffff016602000c82018184010444c000020101000103a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8

    P1 signature = 11186cdc78c6437bfdc9761afe9096633b02dd684ab8bafadc78516c7da00e91c5e838865f53f111ca3ce0ab05e86d38e0534e98476c38567f6168d29b2b320e

The complete P1 input is exactly unsigned record || signature. Ed25519.Verify(public, unsigned, signature) must return true. Under a **future accepted successor State profile** with matching Network and current valid Epoch, it is a positive record candidate; full View acceptance additionally needs valid authority, time, committed accepted/rejected roots, materialization and ClosedProfile linkage. Under current ardents-route-v3, it is rejected as unknown ARNR schema.

## Negative vectors

| ID | Construction and exact signature | Expected record-level decision under proposed successor |
|---|---|---|
| N1 | Start from complete P1, replace the unique address bytes c0000201 with c1000201, retain P1 signature. The CBOR remains structurally canonical. | Invalid Ed25519 signature; no tuple from the changed record is authorized. |
| N2 | Sign the complete 145-byte unsigned record below. Version 1 is encoded as 18 01 instead of shortest 01; length becomes 000d. Signature below is valid for these exact noncanonical bytes. | Malformed/noncanonical EndpointSet, despite valid signature. Do not normalize and then accept. |
| N3 | Start from P1 unsigned, replace its 32-byte Network at offsets 5..36 with 0x12 repeated 32 times, and use the N3 signature below. | Signature valid, Network mismatch against the fixed 0x11 Network; reject before View inclusion. |
| N4 | Submit complete P1 under the current ardents-route-v3 consumer. | Reject unknown schema 3 before State acceptance; never reinterpret as schema 2 text. |
| N5 | Submit a correctly signed schema-2/v2-Carrier record under the proposed successor profile. | Reject old record format for successor duty even if the signer/key is known; preserve it only as historical evidence. |

N2 exact unsigned record:

    41524e52031111111111111111111111111111111111111111111111111111111111111111222222222222222222222222222222222222222222222222222222222222222200000000000000010000000000000000000000007fffffff016602000d8218018184010444c000020101000103a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8

    N2 signature = 454050c58d8fcaf60d75ac97cc83a7687a0ad075207920ee7de07b3c5fdd9850bf036cb8c9ceaf18d70eb221bb6fc507f0652b660b8f0c1ba162f4a6d05a3c0c

    N3 signature = 307766ef01ad368c41ac8dcc1f65031d80ddc7d7bf1f37e02ea62444270d76d26e3158682cc740b443ec08faa3c89ff82267d234eb6d9c60c53919370e811b0f

N2's signature must verify against N2 unsigned bytes and must not verify against P1 unsigned bytes. N3's signature must verify against the precisely modified N3 unsigned bytes and must not verify against P1 unsigned bytes. N1 must fail signature verification. Reject code ordering and the exact successor profile name require the accepted State owner decision; this file does not assert current runtime result codes.

## Reproduction and limits

The vectors were produced with Go 1.26.8 standard crypto/ed25519: seed[i] = byte(i) for i=0..31; unsigned record assembled in the field order above using binary.BigEndian for u16/u64/i64; signature = ed25519.Sign(privateKey, unsigned). The minimal EndpointSet hex was assembled directly from the proposed CBOR grammar and checked for 12-byte length. A separate full State fixture must place positive and negative inputs into one committed input log, compute matching accepted/rejected roots, materializations and ClosedProfile digest under the accepted new profile. A positive signature alone does not prove membership, safe destination, listener reachability or a usable Route.
