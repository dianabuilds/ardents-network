# Hosted-Service Publication And Withdrawal

## Scenario ID

`HSI-002`

## Layer

`integration`

## Domain

`Hosted Services`

## Category

Publication, reachability, request handling, withdrawal, and recovery.

## Goal

Run two real Waku nodes in the Linux test container. The publisher owns a real
generation-aware HTTP workload listener with distinct loopback probe and private
advertised addresses. Wait for consecutive readiness evidence and publication,
then start the second node from the publisher's Waku endpoints.

The receiving node must import and resolve the signed service record through the
private canonical Waku discovery path, and an HTTP request to the resolved
advertised endpoint must succeed. Stopping the backing workload must publish a
withdrawal within the bounded refresh interval; the receiving node must resolve
`not_found`, and the direct request must fail.

## Related Tests

- `tests/integration/hosted-services/publication_test.go::TestPublishedServiceResolvesAndConnectsAcrossRealWakuNodes`
