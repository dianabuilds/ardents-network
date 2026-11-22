package main

import "testing"

func TestCollectorEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:0", "http://127.0.0.1:65536", "http://example.com:4318", "http://localhost:4318", "http://127.0.0.1:4318/?token=secret", "http://user:pass@127.0.0.1:4318", "https://127.0.0.1:4318"} {
		if collectorEndpoint(endpoint) == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
}

func TestObservationWithoutCollectorNeedsNoExporter(t *testing.T) {
	observer, err := newObservation("")
	if err != nil || observer != nil {
		t.Fatalf("local-only observation allocated an exporter: %v, %v", observer, err)
	}
	observer.begin("ardents.hosting", "hosting.observe")("execute", "completed")
}
