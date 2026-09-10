//go:build linux

package endpoint

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// A real bootstrap TLS flight is held by a silent peer. Only job retirement
// cancels the operation; the caller and independently authorized context live.
func TestTextIntroductionPreparationRetirementInterruptsBootstrap(t *testing.T) {
	endpoint, owner, source := textSourceContextFixture(t)
	prepareTextIssuancePermission(t, owner, source)
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	for index := range source.snapshot.Candidates[:source.snapshot.CandidateCount] {
		source.snapshot.Candidates[index].Endpoint = listener.Addr().String()
	}
	source.mu.Unlock()
	job := liveTextCapsuleJob(t, owner)
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	now := time.Now()
	go func() {
		_, err := owner.prepareTextIntroduction(caller, job, targetlink.Link{Network: endpoint.network, Target: fixtureID(199)},
			[3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()})
		result <- err
	}()
	joined := false
	defer func() {
		cancel()
		if !joined {
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("preparation cleanup did not join")
			}
		}
	}()
	peer, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := peer.Read(make([]byte, 4096)); n == 0 || err != nil {
		t.Fatalf("bootstrap TLS did not start: %v", err)
	}
	owner.retireJob(job)
	select {
	case err := <-result:
		joined = true
		if err == nil {
			t.Error("retired preparation succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Error("worker retirement left bootstrap waiting for peer")
	}
	if caller.Err() != nil || owner.lease.Context().Err() != nil {
		t.Error("test cancelled broader authority instead of only the job")
	}
	owner.mu.Lock()
	if joined && (owner.prefixOpening != nil || owner.issuance != nil || owner.resolution != nil) {
		t.Error("preparation returned before joining its network flights")
	}
	owner.mu.Unlock()
}
