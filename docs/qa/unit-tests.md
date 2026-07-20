# Unit Tests Inventory

## Назначение

Этот документ фиксирует unit-level coverage в краткой форме.

Для `unit` tests допускается inventory-style описание одной строкой, если:

- тесты не реализуют multi-step runtime scenario;
- тесты не пересекают реальные runtime boundaries;
- тесты не требуют отдельного integration/e2e scenario document.

## Inventory

- `application/workload/admission`: `gate_test.go::TestGateCheckDelegatesToPolicy`; `gate_test.go::TestGateFuncAllowsMissingPolicy`.
- `application/workload/runtime`: `service_test.go::TestRegisterPresentWorkloadRemainsAccepted`; `service_test.go::TestUnsupportedKindFailsAdmission`; `service_test.go::TestRepeatedStartFailureBecomesFailed`; `service_test.go::TestInspectFailureDegradesWithoutRestart`.
- `infrastructure/workload/runner`: `process_executor_test.go::TestWaitForWorkloadStopReturnsErrorWhenProcessStaysRunning`; `process_executor_test.go::TestProcessRunningTracksStartedProcess`.
- `domain/hosting`: `truth_test.go::TestPublishedServiceSpecsIncludesOnlyRuntimePublishedWorkloads`; `truth_test.go::TestEffectivePublishedServicesMarksDeniedPublication`; `truth_test.go::TestHostedServiceStatusForIDUsesEffectivePublicationTruth`; `truth_test.go::TestPublicationPlanSeparatesAllowedAndDeniedServices`.
- `application/network`: `transport_test.go::TestBuildCandidates`; `transport_test.go::TestBuildCandidatesUsesObservedMultiaddrReachability`; `transport_test.go::TestTransportEndpointsExposeLibp2pMultiaddr`; `transport_test.go::TestTransportEndpointsDoNotExposeQUICOrWebTransport`; `startup_profiles_test.go::TestLibP2POptionsForDefinitionSupportsTCPWSS`; `health_test.go::TestServicePartSnapshotReflectsRuntimeAssessment`; `wss_test.go::TestSecureWebsocketAddressFallsBackToBindAddress`.
- `domain/network`: `bootstrap_status_test.go::TestClassifyBootstrapStatusRequiresOperationalRelayPath`; `profile_runtime_test.go::TestLookupProfileReturnsTCPWSSDefinition`; `profile_runtime_test.go::TestRuntimeShapeForProfileRejectsUnimplementedProfile`.
- `infrastructure/network`: discovery publish filtering, relay envelope mapping,
  and progress-on-failure behavior.
- `application/node`: `bootstrap_support_test.go::TestNetworkBootstrapSourcesFiltersMultiaddrs`; lifecycle snapshot shaping, snapshot/helper mapping, and query-service coordination for node-owned read orchestration.
- `application/identity`, `domain/identity/*`, `infrastructure/identity`: canonical subject normalization, authorization decisions, corrupt-ledger rejection, and private-key restore from the keystore.
- `application/discovery`, `domain/discovery/*`, `infrastructure/discovery`: persisted record/state restore, canonical record validation, intake merge rules, trust evaluation, resolution outcomes, and degraded-state reason retention.
- `infrastructure/runtime/publication`: mapping helpers and thin-layer status behavior.
- `domain/network/route`: route preview helpers do not mutate authoritative route state.
- `application/data`, `infrastructure/data/state`, `infrastructure/data/payload`: object/blob/manifest persistence; grouped `Service` capability facades; metadata/content identity enforcement; retention flows; encrypted payload storage; relay retention accounting; persisted source/transfer ledgers; stale remote source projection; remote fetch and peer-assisted re-serving.
- `domain/data/observed`: `observed_test.go::TestInventoryProjectsObservedLocalAndRelayTruth`; `observed_test.go::TestReconcileLoadedBlobsProjectsExpiredAndMissingPayloadState`.
- `application/data/transfer`: `fetch_test.go::TestAcceptBlobResponseRejectsUnsignedSpoofedSource`; `fetch_test.go::TestAcceptBlobResponseReturnsSignedTerminalError`; `fetch_test.go::TestAcceptBlobResponseRejectsSignedMismatchedContentIdentity`; `fetch_test.go::TestAwaitBlobFetchResponseReturnsCandidateRejectionInsteadOfTimeout`.
- `application/policy`: `service_test.go::TestPolicyAllowDenyMatrix`; `service_test.go::TestPolicySnapshotStaysEnforcedAfterSubsequentAllows`; `service_test.go::TestPolicyUsesOneNormalizationRuleAcrossSurfaces`.
- `domain/policy/evaluation`: `service_publication_test.go::TestServicePublicationDecision`; `service_publication_test.go::TestEffectivePublishedServicesRemainsConsumerOwned`; `routes_test.go::TestCheckRouteUse`.
- `domain/policy/enforcement`: `snapshot_test.go::TestSnapshotCarriesStateAndReason`.
- `domain/diagnostics`: recorder load/recovery, corrupt-ledger handling, active-vs-retained health separation, snapshot restoration, persistence health.
- `infrastructure/runtime/authority`: discovery trust diagnostics projection stays authoritative and query-free.
- `domain/diagnostics/health`: summary composition and retained explainability overlay.
- `domain/diagnostics/operations`: persisted operation normalization and pending-order stability.
- `domain/diagnostics/redaction`: nested sensitive payload redaction.
- `domain/diagnostics/persistence`: ledger save/load round-trip, closed-operation compaction, and corrupt-ledger decode handling.
- `internal/transport/connectrpc`: `auth_test.go::TestAuthConfigValidateRequiresTokenAndScopes`; `auth_test.go::TestAuthConfigCallContextValidatesAuthorizationHeader`.
- `boundary/cli`: `config_test.go::TestConfigResolveReadsContextFileAndEnvOverrides`; `client_test.go::TestServiceClientUsesBearerToken`; `run_test.go::TestRunNodeStatusJSONSuccess`; `run_test.go::TestRunDiagnosticsHealthHumanSuccess`; `run_test.go::TestRunNodeStatusUnauthorizedFailure`; `run_test.go::TestRunWorkloadListSuccess`; `run_test.go::TestRunDataInventoryJSONSuccess`; `run_test.go::TestRunNodeStatusHumanIncludesOperatorTruth`; `run_test.go::TestRunJSONFailureWritesStructuredErrorToStderrOnly`; `run_test.go::TestRunDiagnosticsHealthWatchPrintsInitialSnapshot`; `run_test.go::TestRunNetworkStatusWatchJSONPrintsSnapshotDocument`; `diagnostics_render_test.go::TestRenderDiagnosticsPendingHumanIncludesRecoveryContext`; `diagnostics_render_test.go::TestRenderDiagnosticsExplainHumanIncludesReasonDetailAndNextSteps`; `watch_test.go::TestWatchSnapshotsHumanShowsRetryRecoveryAndUpdatedTruth`; `watch_test.go::TestWatchSnapshotsBudgetExhaustionFailsExplicitly`; `watch_test.go::TestWatchSnapshotsDegradedTruthDoesNotLookLikeTransportRetry`; `tui_test.go::TestTUISectionNavigationWraps`; `tui_test.go::TestTUIModelViewHighlightsActiveTabAndSnapshot`; `tui_test.go::TestTUIModelArrowNavigationChangesActiveSection`; `tui_test.go::TestTUIActionForKeyFollowsDocumentedSectionsOnly`; `error_test.go::TestBuildCLIErrorUsesStructuredAPIErrorDetail`; `error_test.go::TestRenderErrorHumanPrintsStructuredFields`; `cli_input_test.go::TestParseFileArgRequiresFileFlag`; `cli_input_test.go::TestLoadProtoJSONReadsFile`; `cli_input_test.go::TestFirstArgRejectsEmptyInput`; `cli_input_test.go::TestFormatStructSortsKeysDeterministically`.

## Правило актуализации

После добавления новой unit group этот inventory должен обновляться в том же
slice, что и код.


