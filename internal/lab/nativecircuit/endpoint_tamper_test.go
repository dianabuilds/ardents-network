package nativecircuit

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestJoinedRouteRejectsModifiedProtectedRecord(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	certificate, trust := newEndpointFixture(t, "tamper-instance")
	user, proxyUser := net.Pipe()
	proxyService, service := net.Pipe()
	nonce := randomHandle(t)
	serviceDone := make(chan error, 1)
	go func() {
		_, err := runEndpointService(ctx, service, certificate, nonce)
		serviceDone <- err
	}()

	var armed sync.Once
	ready := make(chan struct{})
	var modified atomic.Bool
	proxyDone := make(chan error, 2)
	go func() {
		encrypted := 0
		proxyDone <- copyNativeTLSRecords(proxyUser, proxyService, func(recordType byte, _ []byte) {
			if recordType == 23 {
				encrypted++
				if encrypted == 2 {
					armed.Do(func() { close(ready) })
				}
			}
		})
	}()
	go func() {
		proxyDone <- copyNativeTLSRecords(proxyService, proxyUser, func(recordType byte, payload []byte) {
			if recordType != 23 || len(payload) == 0 || modified.Load() {
				return
			}
			select {
			case <-ready:
				payload[len(payload)-1] ^= 1
				modified.Store(true)
			default:
			}
		})
	}()
	observation, err := runEndpointUser(ctx, user, trust, nonce, []byte("must remain unauthenticated"))
	_ = proxyUser.Close()
	_ = proxyService.Close()
	for range 2 {
		<-proxyDone
	}
	<-serviceDone
	if err == nil || !modified.Load() {
		t.Fatalf("modified protected record was not rejected: modified=%v err=%v", modified.Load(), err)
	}
	if observation.ApplicationBytesVerified {
		t.Fatal("modified protected bytes reached the Application stream")
	}
}
