package runtimeevent

import "slices"

// Recorded schema identifiers retain their spelling for existing captures.
const (
	// Unclassified is the safe projection of an unsupported failure category.
	Unclassified   = "unclassified"
	NodeSchema     = "ardents-node-event-v1"
	SourceSchema   = "ardents-source-event-v1"
	EndpointSchema = "ardents-headless-runtime-event-v1"
	ResourceSample = "resource-sample"
)

var nodeKinds = []string{"lifecycle", "resource", ResourceSample}
var sourceKinds = []string{"source-ready", "source-wave-accepted", "source-failed"}
var endpointKinds = []string{
	"headless-runtime-ready",
	"headless-runtime-failed",
	"headless-runtime-permission-required",
	"headless-runtime-publication-refresh-failed",
	"headless-runtime-publication-withdrawal-failed",
	"headless-runtime-connection-operation-failed",
}
var endpointFailures = []string{
	"startup", "running", "rotation", "authorization", "publication-state",
	"publication-handover", "caller-context", "publisher-ended", "publication-draining",
	"registration-absent", "publisher-not-live", "registration", "publisher-drain",
	"deadline", "admission", "activation", "worker-launch", "worker-operation",
	"introduction-preparation", "service-join", "post-join-lifetime", "service-result",
	"rotation-authority", "rotation-prefix", "rotation-recipient", "rotation-expired",
	"rotation-registration", "rotation-publication",
}
var states = []string{
	"ABSENT", "PREPARED", "READY", "DRAINING", "WITHDRAWN", "FAILED", "OBSERVED",
	"NORMAL", "PROTECT", "DRAIN", "EXIT",
}

// Category binds one recorded event kind to its original schema.
type Category struct{ Schema, Kind string }

// Categories returns a detached catalogue for decoding, summaries and series.
func Categories() []Category {
	var result []Category
	for _, group := range []struct {
		schema string
		kinds  []string
	}{
		{NodeSchema, nodeKinds}, {SourceSchema, sourceKinds}, {EndpointSchema, endpointKinds},
	} {
		for _, kind := range group.kinds {
			result = append(result, Category{group.schema, kind})
		}
	}
	for _, report := range commandReports {
		result = append(result, Category{CommandReportSchema, report.operation})
	}
	return result
}

// KnownKind reports membership in the selected recorded format.
func KnownKind(schema, kind string) bool {
	for _, category := range Categories() {
		if category.Schema == schema && category.Kind == kind {
			return true
		}
	}
	return false
}

// KnownFailure admits only the finite categories permitted in safe projections.
// Node reasons are local free-form details and are never safe failure labels.
func KnownFailure(schema, value string) bool {
	switch schema {
	case SourceSchema:
		return value == "background-work" || value == "cleanup"
	case EndpointSchema:
		return slices.Contains(endpointFailures, value)
	default:
		return false
	}
}

// KnownState admits only the retained finite state vocabulary for export.
func KnownState(value string) bool { return slices.Contains(states, value) }

// KnownCarrier recognizes recorded profiles, without admitting a live Carrier.
func KnownCarrier(value string) bool {
	return value == "ardents-carrier-tcp-tls-v2" || value == "ardents-carrier-quic-v2"
}
