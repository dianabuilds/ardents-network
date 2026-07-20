# Encrypted Private Relay Envelope

- Scenario ID: `NPI-002`
- Layer: Integration
- Domain: Network Foundation / Messaging + Identity
- Category: Security / messaging / recovery

## Goal

Prove that two independently persisted nodes can exchange an
`ardents-private/1` envelope through real Waku Relay while the carrier sees
only an opaque selector and ciphertext, and that authenticated replay remains
rejected after replay-ledger restart.

## Preconditions

- two real local Waku transports are started and connected;
- both nodes hold distinct recipient-bound grants for the same channel
  generation and secret;
- the receiver has imported the sender's trusted signed grant;
- both capability stores and the receiver replay ledger use distinct protected
  local persistence paths.

## Steps

1. Resolve publish and subscribe capability material through Identity/Policy.
2. Subscribe the receiver to the derived opaque content topic.
3. Sign, pad, and encrypt a discovery-class payload before Waku publication.
4. Publish the outer envelope through Waku Relay and capture the carrier bytes.
5. Authenticate, replay-admit, authorize, decrypt, and deliver the payload.
6. Reopen the durable replay ledger and submit the same captured envelope.
7. Tamper with captured ciphertext and submit it through a fresh replay ledger.
8. Mutate the captured opaque selector and encrypted payload into readable
   equivalents and run the canonical carrier-capture detector against both.

## Expected Result

- the receiver obtains the exact authenticated domain payload and sender;
- captured topic/payload contains none of the tested principal, service,
  request, class, or plaintext payload semantics;
- the exact duplicate fails with `privacy.envelope.replayed` after restart;
- tampered ciphertext fails with
  `privacy.envelope.authentication_failed` and yields no plaintext.
- readable selector and plaintext payload mutations are detected as
  `privacy.capture.opaque_selector_invalid` / `readable_topic` and
  `privacy.capture.encrypted_payload_invalid` / `readable_payload`.

## Failure/Degraded Variant

Replay persistence failure, missing sender authority, topic relocation,
authentication failure, or replay-capacity exhaustion is terminal and does not
fall back to a readable topic or plaintext delivery.

## Related Tests

- `tests/integration/network-foundation/privacy_envelope_test.go::TestPrivateEnvelopeTraversesWakuRelayAndRejectsRestartReplay`

## False Positive Risk

The scenario would be weak if it invoked Seal/Open locally without exercising
Waku. It starts two real transports, subscribes on the opaque topic, publishes
the actual ciphertext, and asserts the exact captured carrier bytes.

## False Negative Risk

The test uses bounded testkit readiness waits, a whole-second clock, stable
capability inputs, and the canonical transport bootstrap so unrelated timing
or random identifiers do not drive assertions.

## Notes

Discovery payload interpretation remains outside Network Foundation. This
scenario proves secure framing and delivery only; STB-204 owns migration of
discovery/publication semantics.
