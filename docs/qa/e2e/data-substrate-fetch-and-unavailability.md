## Scenario ID

`DAE-001`

## Layer

`e2e`

## Domain

`Data Substrate`

## Category

`remote fetch / degraded availability`

## Goal

Prove that the data substrate exposes remote encrypted blob availability through
real runtime boundaries and keeps failed fetches operator-visible without
creating false local availability.

## Preconditions

- a source node starts from an isolated data dir that already contains one
  encrypted blob;
- a trusted requester starts with the source public key in trust anchors;
- an untrusted requester starts with the same bootstrap endpoints but without
  the source trust anchor.

## Steps

1. Start the source node and obtain its published discovery bootstrap
   endpoints.
2. Start a trusted requester and fetch the blob through the canonical local
   data control surface.
3. Read the fetched blob and data inventory through the requester control
   surface, then verify the payload remains encrypted at rest.
4. Start an untrusted requester and attempt to fetch the same blob through the
   canonical local data control surface.
5. Read the failed requester inventory and blob lookup state through the same
   control surface.

## Expected Result

- the trusted requester fetches the blob successfully from a real remote node;
- the fetched blob remains encrypted at rest and is decryptable only with the
  valid key material;
- the untrusted requester receives an explicit terminal fetch error instead of a
  silent timeout;
- the untrusted requester does not gain a false local blob copy and its
  inventory remains honest about unavailability.

## Failure/Degraded Variant

- if trusted fetch creates plaintext-at-rest retention, encrypted substrate
  truth is broken;
- if untrusted fetch fails only by timeout, operator-visible failure truth is
  too weak;
- if the failed requester still reports a local blob copy, data availability
  truth becomes false.

## Related Tests

- `tests/e2e/data-substrate/fetch_test.go::TestDataSubstrateRemoteFetchAndUnavailableTruth`

## False Positive Risk

- asserting only a successful fetch response can miss plaintext-at-rest drift;
- asserting only the failed error can miss a silently persisted local blob copy.

## False Negative Risk

- the scenario must use bounded waits so failures map to data/runtime truth
  rather than arbitrary network delay;
- the degraded branch must reject context timeout as an acceptable terminal
  explanation.

## Notes

- deeper retention/restart reconciliation remains covered by `DAI-001`
  integration coverage;
- this e2e slice closes the remaining operator-facing remote availability gap
  for Slice 4.
- remote source and transfer truth must come from `Data Substrate` itself; the
  scenario is not satisfied by diagnostics-only reconstruction.
