# Private Store Revocation

- Scenario ID: `NPI-003`
- Layer: integration
- Domain: Network Foundation / Messaging + Identity
- Category: Security / Store / revocation

## Goal

Prove that Waku Store may retain an encrypted discovery envelope, while current
capability authority still prevents delivery after the sender grant is revoked.

## Expected Result

- Store and its operator see only the opaque content topic and ciphertext;
- a retained envelope from a revoked sender yields no discovery entry;
- the rejection is classified as a privacy authorization failure;
- no readable discovery topic or plaintext compatibility path is used.

## Related Tests

- `tests/integration/network-foundation/transport_store_test.go::TestPrivateStoreRejectsSenderRevokedAfterPublication`

## False Positive Risk

Deleting retained data could look like successful revocation enforcement. The
scenario keeps the encrypted Store record available and requires rejection by
current capability authority during intake.

## False Negative Risk

Store retention is asynchronous. The test first proves retained retrieval,
then applies revocation and evaluates the same envelope through a bounded path.
