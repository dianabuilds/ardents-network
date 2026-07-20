# External Waku Privacy Capture

- Scenario ID: `E2E-NPI-001`
- Layer: E2E
- Domain: Network Privacy
- Category: Security / carrier observation

## Goal

Prove at the privacy-channel-to-real-Waku boundary that an external carrier
observer sees only the fixed private envelope shape, opaque selector, and
ciphertext while product principal, service, and operation meaning remains
absent.

## Preconditions

- two real local Waku transports are connected;
- sender and receiver hold distinct authorized grants for one discovery realm;
- the capture is taken from the receiver's raw Waku subscription before Open.

## Expected Result

- the raw envelope passes the canonical selector and encrypted-payload shape
  validators;
- the topic and payload contain none of the exact tested principal, service,
  operation, or full plaintext markers;
- no readable compatibility envelope is emitted.

## Related Tests

- `tests/e2e/network-privacy/carrier_capture_test.go::TestExternalWakuCaptureContainsOnlyPrivateCarrierShape`

## False Positive Risk

The test captures from a real Waku subscriber rather than inspecting a local
pre-publication object.

## False Negative Risk

The assertion proves absence only for the declared semantic markers and fixed
carrier shape; it does not claim traffic-analysis or size-correlation
anonymity.
