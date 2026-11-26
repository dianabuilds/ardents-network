package timeline

// Retained event schemas and categories are compatibility inputs to the local
// timeline. Keep their recorded spelling when runtime producers are retired.
// These constants describe accepted input, not a shared runtime event API.
const (
	nodeEventSchema     = "ardents-node-event-v1"
	sourceEventSchema   = "ardents-source-event-v1"
	endpointEventSchema = "ardents-headless-runtime-event-v1"

	nodeLifecycle      = "lifecycle"
	nodeResource       = "resource"
	nodeResourceSample = "resource-sample"

	sourceReady          = "source-ready"
	sourceWaveAccepted   = "source-wave-accepted"
	sourceFailed         = "source-failed"
	sourceBackgroundWork = "background-work"
	sourceCleanup        = "cleanup"

	endpointReady                       = "headless-runtime-ready"
	endpointPermissionRequired          = "headless-runtime-permission-required"
	endpointFailed                      = "headless-runtime-failed"
	endpointPublicationRefreshFailed    = "headless-runtime-publication-refresh-failed"
	endpointPublicationWithdrawalFailed = "headless-runtime-publication-withdrawal-failed"
	endpointConnectionOperationFailed   = "headless-runtime-connection-operation-failed"
)
