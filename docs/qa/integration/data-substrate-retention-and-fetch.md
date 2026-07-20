# Scenario DAI-001

- `Layer`: `integration`
- `Domain`: `Data Substrate`
- `Category`: `retention / fetch / recovery`

## Goal

РџРѕРґС‚РІРµСЂРґРёС‚СЊ, С‡С‚Рѕ `Data Substrate` СЃРѕС…СЂР°РЅСЏРµС‚ object/blob truth С‡РµСЂРµР· restart,
РІРѕСЃСЃС‚Р°РЅР°РІР»РёРІР°РµС‚ retention state Р±РµР· Р»РѕР¶РЅРѕР№ local availability, РѕР±СЉСЏСЃРЅСЏРµС‚
broken metadata/payload truth РІ node snapshot Рё РјРѕР¶РµС‚ Р·Р°Р±РёСЂР°С‚СЊ encrypted blob СЃ trusted peer,
РѕС‚РІРµСЂРіР°СЏ unusable remote source.

## Preconditions

- РґРѕСЃС‚СѓРїРµРЅ single-node persisted runtime path РґР»СЏ restart recovery;
- РґРѕСЃС‚СѓРїРµРЅ multi-node path РїРѕРІРµСЂС… canonical transport РґР»СЏ blob fetch;
- requester РјРѕР¶РµС‚ СЂР°Р·Р»РёС‡Р°С‚СЊ trusted Рё untrusted remote peer;
- data diagnostics РґРѕСЃС‚СѓРїРЅС‹ С‡РµСЂРµР· node-managed surfaces.

## Steps

1. Р—Р°РїСѓСЃС‚РёС‚СЊ node СЃ data dir, РѕРїСѓР±Р»РёРєРѕРІР°С‚СЊ blob Рё РїРµСЂРµРІРµСЃС‚Рё РµРіРѕ РІ expired
   retention state.
2. РџРµСЂРµР·Р°РїСѓСЃС‚РёС‚СЊ node РёР· С‚РѕРіРѕ Р¶Рµ data dir.
3. РџСЂРѕРІРµСЂРёС‚СЊ restored blob state Рё operator-visible inventory counters.
4. РџРѕРґРЅСЏС‚СЊ trusted source node СЃ encrypted blob.
5. РџРѕРґРЅСЏС‚СЊ requester node СЃ trust anchor РЅР° source Рё РІС‹РїРѕР»РЅРёС‚СЊ `FetchBlob`.
6. РџСЂРѕРІРµСЂРёС‚СЊ, С‡С‚Рѕ fetched blob РѕСЃС‚Р°С‘С‚СЃСЏ encrypted at rest Рё decryptable С‚РѕР»СЊРєРѕ
   СЃ РІР°Р»РёРґРЅС‹Рј key.
7. РџРѕРґРЅСЏС‚СЊ untrusted requester Рё РїРѕРІС‚РѕСЂРёС‚СЊ fetch РґР»СЏ С‚РѕРіРѕ Р¶Рµ blob.
8. РЎР»РѕРјР°С‚СЊ local payload truth РІ running node Рё РїСЂРѕРІРµСЂРёС‚СЊ, С‡С‚Рѕ `Snapshot().Blob`
   РїРµСЂРµС…РѕРґРёС‚ РІ degraded СЃ blob-specific reason.
9. РЎР»РѕРјР°С‚СЊ persisted metadata truth РјРµР¶РґСѓ restart-Р°РјРё Рё РїСЂРѕРІРµСЂРёС‚СЊ, С‡С‚Рѕ
   `Snapshot().Object` РїРµСЂРµС…РѕРґРёС‚ РІ degraded СЃ missing-blob explanation.

## Expected Result

- restart recovery РїРµСЂРµРІРѕРґРёС‚ expired blob РІ terminal non-local state Р±РµР·
  Р»РѕР¶РЅРѕРіРѕ `available-local`;
- inventory РѕС‚СЂР°Р¶Р°РµС‚ РѕС‚СЃСѓС‚СЃС‚РІРёРµ retained/local truth РїРѕСЃР»Рµ reconcile;
- trusted fetch СЃРѕС…СЂР°РЅСЏРµС‚ blob Р»РѕРєР°Р»СЊРЅРѕ РєР°Рє encrypted payload Рё РґРµР»Р°РµС‚ РµРіРѕ
  РґРѕСЃС‚СѓРїРЅС‹Рј С‡РµСЂРµР· data domain;
- untrusted requester РЅРµ РїРѕР»СѓС‡Р°РµС‚ usable local blob copy Рё РІРёРґРёС‚ explicit trust rejection instead of timeout;
- blob part snapshot РѕС‚СЂР°Р¶Р°РµС‚ lost local payload truth, Р° object part snapshot РѕС‚СЂР°Р¶Р°РµС‚
  broken metadata refs РїРѕСЃР»Рµ restart, РІРјРµСЃС‚Рѕ РґРµРєРѕСЂР°С‚РёРІРЅРѕРіРѕ `ready`.

## Failure/Degraded Variant

- РµСЃР»Рё restart РѕСЃС‚Р°РІР»СЏРµС‚ expired blob РєР°Рє local available, РѕРїРµСЂР°С‚РѕСЂ РїРѕР»СѓС‡Р°РµС‚
  Р»РѕР¶РЅСѓСЋ РґРѕСЃС‚СѓРїРЅРѕСЃС‚СЊ РґР°РЅРЅС‹С…;
- РµСЃР»Рё trusted fetch СЃРѕС…СЂР°РЅСЏРµС‚ plaintext path РёР»Рё С‚РµСЂСЏРµС‚ ciphertext semantics,
  РґРѕРјРµРЅ РЅР°СЂСѓС€Р°РµС‚ encrypted retention model;
- РµСЃР»Рё untrusted fetch РґР°С‘С‚ local copy, trust gating data source broken;
- РµСЃР»Рё broken payload/metadata truth РѕСЃС‚Р°С‘С‚СЃСЏ `ready`, operator surface
  РїСѓР±Р»РёРєСѓРµС‚ РґРµРєРѕСЂР°С‚РёРІРЅС‹Рµ part snapshots РІРјРµСЃС‚Рѕ owner-backed explainability.

## Related Tests

- `tests/integration/data-substrate/node_domain_test.go::TestDataSubstrateObjectAndBlobPersistAcrossRestart`
- `tests/integration/data-substrate/node_domain_test.go::TestDataSubstrateRestartReconcilesMissingPinnedPayloadBlob`
- `tests/integration/data-substrate/node_domain_test.go::TestDataSubstrateOperationsEmitDiagnosticsEvents`
- `tests/integration/data-substrate/node_domain_test.go::TestDataSubstrateRejectsPlaintextRemoteReserve`
- `tests/integration/data-substrate/node_domain_test.go::TestDataSubstrateBlobResponseRequiresDiscoveredRequester`
- `tests/integration/data-substrate/domain_test.go::TestDataSubstrateRestartReconcilesExpiredRetention`
- `tests/integration/data-substrate/domain_test.go::TestDataSubstrateFetchesEncryptedBlobFromTrustedPeer`
- `tests/integration/data-substrate/domain_test.go::TestDataSubstrateRejectsFetchFromUntrustedPeer`
- `tests/integration/data-substrate/domain_test.go::TestDataSubstrateSnapshotExplainsBlobPayloadLoss`
- `tests/integration/data-substrate/domain_test.go::TestDataSubstrateSnapshotExplainsBrokenObjectRefsAfterRestart`

## False Positive Risk

- restart path РјРѕР¶РµС‚ РїСЂРѕР№С‚Рё С‚РѕР»СЊРєРѕ РїРѕ С„Р°РєС‚Сѓ СѓСЃРїРµС€РЅРѕРіРѕ `Start()`, РЅРµ РїСЂРѕРІРµСЂРёРІ
  final blob state Рё inventory counters;
- trusted fetch РјРѕР¶РµС‚ РїСЂРѕР№С‚Рё, РЅРµ РґРѕРєР°Р·Р°РІ, С‡С‚Рѕ payload РѕСЃС‚Р°С‘С‚СЃСЏ ciphertext at rest;
- untrusted fetch РјРѕР¶РµС‚ silently preserve local copy, РµСЃР»Рё test РЅРµ РїСЂРѕРІРµСЂСЏРµС‚
  РѕС‚СЃСѓС‚СЃС‚РІРёРµ stored blob;
- part-snapshot checks РјРѕРіСѓС‚ Р»РѕР¶РЅРѕ РїСЂРѕР№С‚Рё, РµСЃР»Рё test РЅРµ РїСЂРѕРІРµСЂСЏРµС‚
  degraded state together with reason text.

## False Negative Risk

- multi-node fetch assertions РЅРµ РґРѕР»Р¶РЅС‹ РѕРїРёСЂР°С‚СЊСЃСЏ РЅР° РЅРµСЃС‚Р°Р±РёР»СЊРЅС‹Р№ timing Р±РµР·
  bounded timeout;
- restart scenario РЅРµ РґРѕР»Р¶РµРЅ Р·Р°РІРёСЃРµС‚СЊ РѕС‚ РІРЅРµС€РЅРµРіРѕ network state;
- fetch timeout РЅРµР»СЊР·СЏ С‡РёС‚Р°С‚СЊ expected denial, РµСЃР»Рё requester СѓР¶Рµ РїРѕР»СѓС‡РёР» candidate
  response СЃ explicit unusable/trust outcome.

## Notes

Remote fetch now uses `ardents-private/1` request/response classes over the
capability-derived data-exchange selector. The inner signed requester and
responder contracts remain owned and validated by Data Substrate; Waku carries
only the opaque encrypted envelope.

РЎС†РµРЅР°СЂРёР№ РїРѕРєСЂС‹РІР°РµС‚ explicit integration layer РґР»СЏ data domain; package-local
`internal/data/data_test.go` РѕСЃС‚Р°С‘С‚СЃСЏ unit/focused owner coverage, Р° downstream
mixed tests РїРѕРґР»РµР¶Р°С‚ cleanup РЅР° С„РёРЅР°Р»СЊРЅРѕР№ acceptance С„Р°Р·Рµ РґРѕРјРµРЅР°.
