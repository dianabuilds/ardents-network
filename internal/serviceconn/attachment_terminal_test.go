package serviceconn

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryExhaustionPublishesTerminalBeforeWakingWaiter(t *testing.T) {
	application, applicationPeer := net.Pipe()
	transport, transportPeer := net.Pipe()
	defer applicationPeer.Close()
	defer transportPeer.Close()
	failed := &securedAttachment{transport: transport, generation: 1}
	firstProposal := make(chan struct{})
	release := make(chan struct{})
	var proposals atomic.Int32
	opener := func(context.Context, Recovery) (net.Conn, error) {
		if proposals.Add(1) == 1 {
			close(firstProposal)
			<-release
		}
		return nil, errors.New("injected unavailable attachment")
	}
	now := time.Now()
	stream := newRecoveryStream(t.Context(), application, Credential{}, Recovery{
		NoNewRecoveryAfter: now.Add(time.Minute).Unix(),
	}, nil, true, opener, failed, [32]byte{1}, now, DestinationBinding{}, nil, newResourceObserver())
	results := make(chan error, 2)
	go func() { results <- stream.recoverAttachment(failed) }()
	<-firstProposal
	go func() { results <- stream.recoverAttachment(failed) }()
	close(release)
	for range 2 {
		if err := <-results; err == nil {
			t.Fatal("terminal recovery exhaustion was accepted")
		}
	}
	if got := proposals.Load(); got != proposalLimit {
		t.Fatalf("waiter started a second recovery pass: proposals=%d", got)
	}
	stream.mu.Lock()
	terminal, recovering := stream.terminal, stream.recovering
	stream.mu.Unlock()
	if terminal == nil || recovering {
		t.Fatalf("terminal was not atomically published: terminal=%v recovering=%v", terminal, recovering)
	}
	stream.close()
	_ = applicationPeer.Close()
}
