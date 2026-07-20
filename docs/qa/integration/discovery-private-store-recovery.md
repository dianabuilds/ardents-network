# Private Discovery Store And Restart Recovery

- Scenario ID: `DKI-003`
- Layer: integration
- Domain: Discovery / Publication over Network Foundation
- Category: Private publication / Store / restart

## Goal

Prove the complete product path: a node signs its discovery record, Publication
encrypts it through a capability channel, Waku Relay/Store carries the opaque
envelope, and another node decrypts and imports the signed fact. Then restart
the receiver and retain the imported catalog even when Store history is
classified as replay by its durable ledger.

## Expected Result

- the remote signed record is imported through the real Waku Store path;
- carrier-visible topic and payload do not contain discovery meaning;
- the persisted remote record survives receiver restart;
- replayed history does not erase or falsely invalidate the retained catalog;
- invalid signed records and privacy failures remain explainable degraded paths.

## Related Tests

- `tests/integration/discovery/private_network_test.go::TestPrivateDiscoveryImportsSignedRecordFromWakuStore`

## False Positive Risk

Persisted local discovery state could be mistaken for a network recovery. The
scenario requires publication through the private Waku path and import by a
separate receiver before restart.

## False Negative Risk

Store convergence is asynchronous. The test uses bounded readiness and fetch
deadlines and reports the failed network stage rather than assuming an
immediate retained result.
