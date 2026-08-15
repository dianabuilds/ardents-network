package node

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestDeclaredCapacityAdmitsExistingSlotsAndRefusesExcess(t *testing.T) {
	fixture := newLifecycleFixture(t)
	events := make(chan Event, 16)
	fixture.config.Current = func() (Facts, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan Result, 1)
	go func() { value, _ := Run(ctx, fixture.config); result <- value }()
	waitForState(t, events, "READY")
	established := make([]*tls.Conn, fixture.snapshot.ProbeCapacity)
	for index := range established {
		established[index] = dialProbe(t, fixture)
	}
	excess, err := tls.Dial("tcp", fixture.config.Probe.ListenAddress, probeClientTLS(fixture))
	if err == nil {
		request := encodeProbeRequest(fixture.snapshot, [32]byte{90}, []byte("over capacity"))
		_, writeErr := excess.Write(request)
		response := make([]byte, testProbeHeaderBytes+sha256.Size)
		_, readErr := io.ReadFull(excess, response)
		_ = excess.Close()
		if writeErr == nil && readErr == nil {
			t.Fatal("work above authenticated capacity succeeded")
		}
	}
	request := encodeProbeRequest(fixture.snapshot, [32]byte{91}, []byte("within capacity"))
	if _, err := established[0].Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, testProbeHeaderBytes+sha256.Size)
	if _, err := io.ReadFull(established[0], response); err != nil {
		t.Fatalf("work inside capacity failed: %v", err)
	}
	for _, connection := range established {
		_ = connection.Close()
	}
	cancel()
	select {
	case terminal := <-result:
		if terminal.State != "WITHDRAWN" {
			t.Fatalf("terminal result = %+v", terminal)
		}
	case <-time.After(time.Second):
		t.Fatal("capacity Node did not clean up")
	}
}

func TestEmergencyPressureDrainsAndExitsWithoutNewAdmission(t *testing.T) {
	fixture := newLifecycleFixture(t)
	events := make(chan Event, 32)
	fixture.config.Current = func() (Facts, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	fixture.config.ResourceProfile = "h3-np1-v1"
	fixture.config.ResourceMeasure = func() (resource.Sample, error) {
		return resource.Sample{MemoryBytes: 460 << 20}, nil
	}
	result := make(chan Result, 1)
	go func() { value, _ := Run(context.Background(), fixture.config); result <- value }()
	waitForState(t, events, "READY")
	waitForState(t, events, "DRAIN")
	waitForState(t, events, "EXIT")
	select {
	case terminal := <-result:
		if terminal.State != "WITHDRAWN" {
			t.Fatalf("terminal result = %+v", terminal)
		}
	case <-time.After(time.Second):
		t.Fatal("emergency pressure did not finish cleanup")
	}
	if connection, err := tls.Dial("tcp", fixture.config.Probe.ListenAddress, probeClientTLS(fixture)); err == nil {
		_ = connection.Close()
		t.Fatal("EXIT left Node admission open")
	}
}
