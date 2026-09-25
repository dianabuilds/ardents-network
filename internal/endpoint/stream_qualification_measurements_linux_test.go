//go:build linux

package endpoint

import (
	"strings"
	"testing"
)

func TestQualificationPublisherIncompleteErrorReportsProgressAndRegistrationReason(t *testing.T) {
	err := qualificationPublisherIncompleteError(193)
	message := err.Error()
	for _, want := range []string{"193/256", "producer ended"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestQualificationUbuntu249KeepsParentExitInvariant(t *testing.T) {
	service := textManagerProperties{"RemainAfterExit": {Type: "b", Data: []byte("false")}}
	if !textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("v249 cannot represent its normal parent lifetime")
	}
	for _, version := range []uint16{0, 248, 250, 255} {
		if textEndpointStopsWithMainVersion(service, version) {
			t.Fatalf("accepted missing properties on %d", version)
		}
	}
	service["ExitType"] = textManagerValue{Type: "s", Data: []byte("\"cgroup\"")}
	if textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("unexpected cgroup lifetime accepted")
	}
	delete(service, "ExitType")
	service["RemainAfterExit"] = textManagerValue{Type: "b", Data: []byte("true")}
	if textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("retained parent accepted")
	}
}
