package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamCutoverRejectsOffsetRollback(t *testing.T) {
	t.Parallel()
	failed := &Attachment{generation: 1}
	fresh := &Attachment{generation: 2}
	stream := &Stream{current: failed, sendBase: 10, sendEnd: 20, recvNext: 15}
	rollback := ContinuityPeer{SendBase: 0, SendEnd: 20, ReceiveNext: 9, PeerNonce: [32]byte{2}, LocalNonce: [32]byte{3}}
	if !errors.Is(stream.commitAttachment(failed, fresh, rollback), ErrActiveViolation) {
		t.Fatal("acknowledgement rollback was accepted")
	}
}

func TestStreamReceiveRangesDeliverOnceInOrderAndRejectConflicts(t *testing.T) {
	t.Parallel()
	application := &bufferApplication{}
	stream := &Stream{application: application, ackSignal: make(chan struct{}, 1)}
	if err := stream.acceptData(&Data{Offset: 3, Payload: []byte("def")}, 6); err != nil {
		t.Fatal(err)
	}
	if application.Len() != 0 {
		t.Fatal("out-of-order bytes reached the Application")
	}
	if err := stream.acceptData(&Data{Payload: []byte("abc")}, 6); err != nil {
		t.Fatal(err)
	}
	if application.String() != "abcdef" || stream.recvNext != 6 {
		t.Fatalf("ranges were not delivered once in order: %q offset=%d", application.String(), stream.recvNext)
	}
	if err := stream.acceptData(&Data{Offset: 2, Payload: []byte("cde")}, 6); err != nil {
		t.Fatalf("matching delayed overlap rejected: %v", err)
	}
	if application.String() != "abcdef" {
		t.Fatal("matching overlap was presented twice")
	}
	if err := stream.acceptData(&Data{Offset: 2, Payload: []byte("Xde")}, 6); !errors.Is(err, ErrActiveViolation) {
		t.Fatalf("conflicting authenticated overlap accepted: %v", err)
	}
}

func TestStreamReceiveRangeMetadataStopsAtEightDisjointRanges(t *testing.T) {
	t.Parallel()
	stream := &Stream{application: &bufferApplication{}, ackSignal: make(chan struct{}, 1)}
	for index := range 8 {
		offset := uint64(index*2 + 1)
		if err := stream.acceptData(&Data{Offset: offset, Payload: []byte{byte(index)}}, 32); err != nil {
			t.Fatalf("range %d rejected: %v", index+1, err)
		}
	}
	if len(stream.pending) != 8 {
		t.Fatalf("wrong disjoint range count: %d", len(stream.pending))
	}
	if err := stream.acceptData(&Data{Offset: 17, Payload: []byte{9}}, 32); !errors.Is(err, ErrActiveViolation) {
		t.Fatalf("ninth disjoint range did not terminate safely: %v", err)
	}
}

func TestStreamFullSendQueueUnblocksForReplay(t *testing.T) {
	t.Parallel()
	stream := &Stream{sendData: make([]byte, logicalQueueLimit), sendEnd: logicalQueueLimit, sendNext: logicalQueueLimit}
	if !stream.sendQueueBlockedLocked() {
		t.Fatal("fully transmitted unacknowledged queue did not apply backpressure")
	}
	stream.sendNext = 0
	if stream.sendQueueBlockedLocked() {
		t.Fatal("reattachment replay remained blocked behind the full send queue")
	}
}

func TestStreamRecoveryExhaustionPublishesTerminalBeforeWakingWaiter(t *testing.T) {
	application, applicationPeer := net.Pipe()
	defer applicationPeer.Close()
	failed := &Attachment{generation: 1}
	firstProposal := make(chan struct{})
	release := make(chan struct{})
	var proposals atomic.Int32
	stream := &Stream{ctx: t.Context(), application: application, recovery: Recovery{
		NoNewRecoveryAfter: time.Now().Add(time.Minute).Unix()}, current: failed, continuity: [32]byte{1},
		opener: func(context.Context, Recovery) (*Attachment, error) {
			if proposals.Add(1) == 1 {
				close(firstProposal)
				<-release
			}
			return nil, errors.New("injected unavailable attachment")
		}, resources: func(string, int) uint32 { return 0 }, ackSignal: make(chan struct{}, 1), done: make(chan struct{})}
	stream.cond = sync.NewCond(&stream.mu)
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
}

func TestStreamExchangesInitialContinuityBeforeBidirectionalData(t *testing.T) {
	clientCarrier, publisherCarrier := net.Pipe()
	clientApplication, clientUser := net.Pipe()
	publisherApplication, publisherUser := net.Pipe()
	defer clientUser.Close()
	defer publisherUser.Close()
	connectionContext, exporter, key := [32]byte{1}, [32]byte{2}, [32]byte{3}
	clientAttachment, err := NewAttachment(clientCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherAttachment, err := NewAttachment(publisherCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientAttachment,
		ContinuityKey: key, Authorized: time.Now(), Client: true})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherAttachment,
		ContinuityKey: key, Authorized: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { _, err := client.Run(3, 3); results <- err }()
	go func() { _, err := publisher.Run(3, 3); results <- err }()
	go func() { _, _ = clientUser.Write([]byte("one")) }()
	go func() { _, _ = publisherUser.Write([]byte("two")) }()
	clientRead, publisherRead := make([]byte, 3), make([]byte, 3)
	if _, err := io.ReadFull(clientUser, clientRead); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(publisherUser, publisherRead); err != nil {
		t.Fatal(err)
	}
	if string(clientRead) != "two" || string(publisherRead) != "one" {
		t.Fatalf("unexpected exchange: client=%q publisher=%q", clientRead, publisherRead)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("initial native stream failed: %v", err)
		}
	}
}

func TestStreamBoundedAcceptsTerminalBeforeDirectionalLimit(t *testing.T) {
	clientCarrier, publisherCarrier := net.Pipe()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	connectionContext, exporter, key := [32]byte{4}, [32]byte{5}, [32]byte{6}
	clientAttachment, err := NewAttachment(clientCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherAttachment, err := NewAttachment(publisherCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientAttachment,
		ContinuityKey: key, Authorized: time.Now(), Client: true})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherAttachment,
		ContinuityKey: key, Authorized: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		outcome Outcome
		err     error
	}
	results := make(chan result, 2)
	go func() { outcome, err := client.RunBounded(32, 32); results <- result{outcome, err} }()
	go func() { outcome, err := publisher.RunBounded(32, 32); results <- result{outcome, err} }()
	go func() { _, _ = clientUser.Write([]byte("request")); _ = clientUser.CloseWrite() }()
	go func() { _, _ = publisherUser.Write([]byte("ok")); _ = publisherUser.CloseWrite() }()
	clientRead, publisherRead := make([]byte, 2), make([]byte, 7)
	if _, err := io.ReadFull(clientUser, clientRead); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(publisherUser, publisherRead); err != nil {
		t.Fatal(err)
	}
	if string(clientRead) != "ok" || string(publisherRead) != "request" {
		t.Fatalf("unexpected bounded exchange: client=%q publisher=%q", clientRead, publisherRead)
	}
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("bounded native stream failed: %v", result.err)
		}
		if result.outcome.Accepted >= 32 || result.outcome.Received >= 32 {
			t.Fatalf("terminal was not accepted before bounds: %+v", result.outcome)
		}
	}
}

func TestRecoveryDeadlineStartsAtLastConnectionProgress(t *testing.T) {
	t.Parallel()
	detected := time.Now()
	progress := detected.Add(-4 * time.Second)
	if got, want := recoveryEpisodeDeadline(progress, detected), progress.Add(recoveryLimit); !got.Equal(want) {
		t.Fatalf("deadline=%v want=%v", got, want)
	}
	if got, want := recoveryEpisodeDeadline(time.Time{}, detected), detected.Add(recoveryLimit); !got.Equal(want) {
		t.Fatalf("zero-progress deadline=%v want=%v", got, want)
	}
	terminal := progress.Add(recoveryLimit)
	if got, want := recoveryWorkDeadline(terminal), terminal.Add(-10*time.Millisecond); !got.Equal(want) {
		t.Fatalf("work deadline=%v want=%v", got, want)
	}
}

type bufferApplication struct{ bytes.Buffer }

func (*bufferApplication) Close() error { return nil }

type halfCloseApplication struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func halfClosePair() (*halfCloseApplication, *halfCloseApplication) {
	leftRead, rightWrite := io.Pipe()
	rightRead, leftWrite := io.Pipe()
	return &halfCloseApplication{reader: leftRead, writer: leftWrite}, &halfCloseApplication{reader: rightRead, writer: rightWrite}
}

func (application *halfCloseApplication) Read(value []byte) (int, error) {
	return application.reader.Read(value)
}

func (application *halfCloseApplication) Write(value []byte) (int, error) {
	return application.writer.Write(value)
}

func (application *halfCloseApplication) CloseWrite() error {
	return application.writer.Close()
}

func (application *halfCloseApplication) Close() error {
	return errors.Join(application.reader.Close(), application.writer.Close())
}
