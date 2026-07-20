# Data Chunked Transfer And Resume

## Scenario ID

`DAI-003`

## Layer

`integration`

## Domain

`Data Substrate`

## Goal

Prove that a payload larger than 64 KiB is independently encrypted and
content-addressed in canonical chunks, transferred over the real private Waku
data-exchange path with bounded concurrency, reconstructed in manifest order,
and resumed without re-fetching already verified chunks.

## Preconditions

- two real Waku nodes share an authorized Data Substrate capability channel;
- the requester trusts the source's signed node identity;
- the source owns a valid encrypted chunk manifest and every referenced Blob;
- every request and response stays within the private-envelope size bound.

## Steps

1. Store a multi-chunk plaintext payload on the source.
2. Start the source and bootstrap a trusted requester over real Waku.
3. Fetch the manifest and its chunks through the runtime Data Substrate API.
4. Verify each stored ciphertext CID and decrypt chunks in manifest order.
5. Repeat the same fetch from the requester state.

## Expected Result

- the first fetch transfers every missing chunk with bounded parallelism;
- the reconstructed plaintext exactly matches the source payload;
- only encrypted chunk bytes are retained and re-served;
- the second fetch reports every chunk as resumed and transfers none again;
- aggregate transfer progress reaches a terminal completed state.

## Failure/Degraded Variants

- a corrupt CID, malformed manifest, missing leaf, timeout, cancellation, or
  private-channel rejection cannot publish a complete local manifest;
- verified chunks survive interruption and are reused by the next attempt;
- uncommitted staging chunks are not served and are cleaned after restart.

## Related Tests

- `tests/integration/data-substrate/chunked_transfer_test.go::TestDataSubstrateFetchesAndResumesChunkedPayloadOverPrivateWaku`
- `internal/data/transfer/chunked_fetch_test.go`
- `internal/data/chunking/chunking_test.go`

## False Positive Risk

- an in-memory carrier would not prove the canonical Waku path;
- checking only counts would miss corrupt ciphertext or wrong chunk order;
- a second fetch without explicit fetched/resumed counts could silently repeat
  the whole transfer.

## False Negative Risk

- network formation and capability exchange require a bounded startup window;
- the test must not treat a context deadline as an expected policy rejection.
