//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
)

func TestQualificationOwnerBarrierRequiresEveryParticipant(t *testing.T) {
	owner, err := NewStreamQualificationMeasurements(2)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := owner.finish(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("incomplete owner barrier: %v", err)
	}
	select {
	case <-owner.ready:
		t.Fatal("barrier released without second participant")
	default:
	}
	if err := owner.finish(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.ready:
	default:
		t.Fatal("complete owner barrier did not release")
	}
	if err := owner.finish(context.Background()); err == nil {
		t.Fatal("duplicate completion accepted")
	}
}

func TestQualificationOwnerCannotHideRetiredSibling(t *testing.T) {
	owner, _ := NewStreamQualificationMeasurements(2)
	for _, group := range []string{"/system.slice/reader-a.service", "/system.slice/reader-b.service"} {
		if err := owner.add(group); err != nil {
			t.Fatal(err)
		}
	}
	owner.retire("/system.slice/reader-a.service")
	if _, err := owner.measure(); err == nil {
		t.Fatal("removed sibling was silently omitted")
	}
	if err := owner.add("/system.slice/reader-c.service"); err == nil {
		t.Fatal("replacement reset owner measurement")
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
