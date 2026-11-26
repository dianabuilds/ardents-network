package telemetry

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestCollectorEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:0", "http://127.0.0.1:65536", "http://example.com:4318", "http://localhost:4318", "http://127.0.0.1:4318/?token=secret", "http://user:pass@127.0.0.1:4318", "https://127.0.0.1:4318"} {
		if ValidateCollector(endpoint) == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
		if observer, err := Open(endpoint); err == nil || observer != nil {
			t.Fatalf("opened an invalid endpoint %s", endpoint)
		}
	}
}

func TestObservationWithoutCollectorNeedsNoExporter(t *testing.T) {
	observer, err := Open("")
	if err != nil || observer != nil {
		t.Fatalf("local-only observation allocated an exporter: %v, %v", observer, err)
	}
	observer.Begin(runtimeevent.HostingObserve)(runtimeevent.HostingObserve.Classify("execute", "completed"))
}
