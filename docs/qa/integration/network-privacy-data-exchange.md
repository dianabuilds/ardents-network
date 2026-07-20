# Private Data Request And Response Exchange

- Scenario ID: `NPI-004`
- Layer: integration
- Domain: Data Substrate over Network Foundation / Identity
- Category: Security / encrypted request-response

## Goal

Prove that blob request routing and content identity are carried only inside an
authenticated private envelope, and that receive-time capability revocation
prevents delivery to Data Substrate.

## Preconditions

- two real Waku transports share valid data-exchange capability material;
- request and response use the common opaque capability selector;
- Data Substrate still signs the inner requester/responder contracts.

## Expected Result

- raw Waku topic/payload contains none of the tested blob ID, requester, or
  request ID values;
- an authorized receiver recovers the exact request and rejects the same
  envelope as replay;
- a request published after sender revocation is rejected before domain
  dispatch;
- there is no readable request/response topic fallback.

## Related Tests

- `tests/integration/network-foundation/data_privacy_test.go::TestPrivateDataRequestHidesRoutingAndContentIdentity`
- `tests/integration/network-foundation/data_privacy_test.go::TestPrivateDataExchangeRejectsRevokedRequesterCapability`

## False Positive Risk

Local Seal/Open could bypass the carrier boundary. The scenario captures the
real Waku topic and payload and requires domain dispatch only after authorized
private-envelope intake.

## False Negative Risk

Relay delivery is asynchronous. Bounded waits separate transport convergence,
replay rejection, and receive-time revocation failures.
