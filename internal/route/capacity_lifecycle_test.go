package route

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestCapacityKeepsPressureLifecycleAfterTargetWhileAcceptedWorkLives(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	probe.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var memory atomic.Uint64
	events := make(chan Evidence, 8)
	finished := make(chan error, 1)
	admitted := make(chan struct{}, 1)
	carry := func(_ context.Context, _ Actor, connection net.Conn, evidence Evidence,
		admit func() bool) (Evidence, error) {
		if !admit() {
			return evidence, errAttachmentCapacity
		}
		kind := make([]byte, 1)
		if _, err := io.ReadFull(connection, kind); err != nil {
			return evidence, err
		}
		if kind[0] == 's' {
			admitted <- struct{}{}
			_, err := io.Copy(io.Discard, connection)
			return evidence, err
		}
		return evidence, nil
	}
	go func() {
		_, runErr := serveCapacity(ctx, Actor{Role: "initiator", ListenAddress: address,
			MaximumAttachments: 2, AttachmentTarget: 1, ResourceProfile: "h3-np1-v1",
			ResourceCheck: func() error { return nil }, PressureInterval: 5 * time.Millisecond,
			ResourceMeasure: func() (resource.Sample, error) {
				return resource.Sample{MemoryBytes: memory.Load()}, nil
			}}, func(value Evidence) { events <- value }, Evidence{}, carry)
		finished <- runErr
	}()
	if event := <-events; event.Kind != "ready" {
		t.Fatalf("capacity did not become ready: %+v", event)
	}
	slow, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()
	_, _ = slow.Write([]byte{'s'})
	<-admitted
	quick, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = quick.Write([]byte{'q'})
	quick.Close()
	select {
	case err := <-finished:
		t.Fatalf("capacity stopped lifecycle with accepted work after target: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	memory.Store(460 << 20)
	if event := <-events; event.State != "DRAIN" {
		t.Fatalf("post-target pressure did not drain: %+v", event)
	}
	if event := <-events; event.State != "EXIT" {
		t.Fatalf("post-target drain did not exit: %+v", event)
	}
	if err := <-finished; err == nil {
		t.Fatal("post-target emergency pressure returned success")
	}
}
