package architecture

import "testing"

func TestChannelMeasurementsCannotAdmitSDKOrForeignOwners(t *testing.T) {
	for _, dependency := range []string{
		"go.opentelemetry.io/otel/sdk/metric/metricdata",
		modulePath + "/internal/diagnostics/telemetry",
		modulePath + "/internal/domain/node",
	} {
		if domainMeasurementImportAllowed("internal/domain/route/channel/frame_schedule.go", dependency) ||
			successorImportAllowed("internal/domain/route/channel/frame_schedule.go", dependency) {
			t.Fatalf("channel measurement admitted foreign dependency %s", dependency)
		}
	}
}

// domainMeasurementImportAllowed registers the finite measurement producers,
// capture composition and format adapters. It grants neither a domain import
// nor SDK/network access to the measurement leaf or an offline Custody owner.
func domainMeasurementImportAllowed(source, dependency string) bool {
	switch dependency {
	case "github.com/dianabuilds/ardents-network/internal/diagnostics/measurement":
		switch source {
		case "cmd/ardents-custody/main.go",
			"cmd/ardents-next/main.go",
			"internal/domain/admission/stock/measurement_unix_test.go",
			"internal/domain/admission/stock/permission.go",
			"internal/domain/admission/stock/stock.go",
			"internal/domain/connection/initiator_authentication.go",
			"internal/domain/connection/transfer/measurement_test.go",
			"internal/domain/connection/publisher_authentication.go",
			"internal/domain/connection/transfer/receive_ledger.go",
			"internal/domain/connection/transfer/send_ledger.go",
			"internal/domain/connection/stream_io.go",
			"internal/domain/custody/canonical_envelope.go",
			"internal/domain/custody/measurement_test.go",
			"internal/domain/endpoint/participant_linux.go",
			"internal/domain/enrollment/bundle.go",
			"internal/domain/enrollment/infrastructure_inventory.go",
			"internal/domain/execution/measurement_test.go",
			"internal/domain/execution/runtime/launch.go",
			"internal/domain/execution/supervisor.go",
			"internal/domain/hosting/budget.go",
			"internal/domain/hosting/measurement_unix_test.go",
			"internal/domain/network/source/transport.go",
			"internal/domain/network/state/refresh.go",
			"internal/domain/node/command/role_command.go",
			"internal/domain/publication/runtime/publisher.go",
			"internal/domain/reachability/store.go",
			"internal/domain/release/endpoint/initial_verification.go",
			"internal/domain/release/verification.go",
			"internal/domain/route/channel/child_capacity.go",
			"internal/domain/route/channel/frame_schedule.go",
			"internal/domain/route/channel/lane.go",
			"internal/domain/route/channel/measurement_test.go",
			"internal/domain/route/channel/queue_budget.go",
			"internal/domain/route/channel/session.go":
			return true
		}
	case "github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent":
		switch source {
		case "internal/diagnostics/measurement/capture.go",
			"internal/diagnostics/measurement/capture_test.go",
			"internal/diagnostics/measurement/command.go",
			"internal/diagnostics/telemetry/measurements.go",
			"internal/diagnostics/telemetry/measurements_test.go",
			"internal/diagnostics/timeline/project.go",
			"internal/domain/admission/stock/measurement_unix_test.go",
			"internal/domain/admission/stock/permission.go",
			"internal/domain/admission/stock/stock.go",
			"internal/domain/connection/initiator_authentication.go",
			"internal/domain/connection/transfer/measurement_test.go",
			"internal/domain/connection/publisher_authentication.go",
			"internal/domain/connection/transfer/receive_ledger.go",
			"internal/domain/connection/transfer/send_ledger.go",
			"internal/domain/connection/stream_io.go",
			"internal/domain/custody/canonical_envelope.go",
			"internal/domain/custody/measurement_test.go",
			"internal/domain/endpoint/participant_linux.go",
			"internal/domain/enrollment/bundle.go",
			"internal/domain/enrollment/infrastructure_inventory.go",
			"internal/domain/execution/measurement_test.go",
			"internal/domain/execution/runtime/launch.go",
			"internal/domain/execution/supervisor.go",
			"internal/domain/hosting/budget.go",
			"internal/domain/hosting/measurement_unix_test.go",
			"internal/domain/network/source/transport.go",
			"internal/domain/network/state/refresh.go",
			"internal/domain/node/command/role_command.go",
			"internal/domain/publication/runtime/publisher.go",
			"internal/domain/reachability/store.go",
			"internal/domain/release/endpoint/initial_verification.go",
			"internal/domain/release/verification.go",
			"internal/domain/route/channel/frame_schedule.go",
			"internal/domain/route/channel/measurement_test.go",
			"internal/domain/route/channel/queue_budget.go",
			"internal/domain/route/channel/session.go",
			"internal/diagnostics/command/capture.go",
			"internal/diagnostics/command/report.go",
			"internal/diagnostics/command/view.go":
			return true
		}
	case "github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry":
		switch source {
		case "cmd/ardents-next/main.go":
			return true
		}
	case "go.opentelemetry.io/otel/attribute":
		switch source {
		case "internal/diagnostics/telemetry/measurements.go":
			return true
		}
	case "go.opentelemetry.io/otel/sdk/instrumentation":
		switch source {
		case "internal/diagnostics/telemetry/measurements.go":
			return true
		}
	case "go.opentelemetry.io/otel/sdk/metric/metricdata":
		switch source {
		case "internal/diagnostics/telemetry/measurements.go":
			return true
		}
	case "go.opentelemetry.io/otel/sdk/resource":
		switch source {
		case "internal/diagnostics/telemetry/measurements.go":
			return true
		}
	case "go.opentelemetry.io/proto/otlp/collector/metrics/v1":
		switch source {
		case "internal/diagnostics/telemetry/measurements_test.go":
			return true
		}
	case "google.golang.org/protobuf/proto":
		switch source {
		case "internal/diagnostics/telemetry/measurements_test.go":
			return true
		}
	}
	return false
}
