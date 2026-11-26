package telemetry

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisabledAndFailedOpeningHaveDifferentDeliveryStatus(t *testing.T) {
	var observation *Observation
	if status := observation.CloseWithStatus(nil); status != LocalOnly {
		t.Fatalf("disabled export: %s", status)
	}
	if status := observation.CloseWithStatus(errors.New("private opening detail")); status != Unavailable {
		t.Fatalf("failed opening: %s", status)
	}
}

func TestDeliveryStatusRetainsCollectorRefusal(t *testing.T) {
	for _, tc := range []struct {
		code int
		want DeliveryStatus
	}{{http.StatusNoContent, Delivered}, {http.StatusServiceUnavailable, Unavailable}} {
		t.Run(string(tc.want), func(t *testing.T) {
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code) }))
			defer collector.Close()
			observation, err := Open(collector.URL)
			if err != nil {
				t.Fatal(err)
			}
			observation.Begin(runtimeevent.HostingObserve)(runtimeevent.HostingObserve.Classify("execute", "completed"))
			if status := observation.CloseWithStatus(err); status != tc.want {
				t.Fatalf("delivery=%s want=%s", status, tc.want)
			}
		})
	}
}
