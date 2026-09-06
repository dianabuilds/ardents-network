package connection

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunBoundedReplaysLostSettledTerminalAfterRecovery(t *testing.T) {
	runTerminalRecoveryJourney(t, false, false, 2, 1)
}

func TestRunBoundedReplaysUnacknowledgedDataBeforeTerminalDuringConcurrentHalfClose(t *testing.T) {
	runTerminalRecoveryJourney(t, true, true, 2, 1)
}

func TestReplaySettledTerminalReplaysOutstandingDataBeforeTerminal(t *testing.T) {
	writer, reader := net.Pipe()
	defer writer.Close()
	defer reader.Close()
	attachment := terminalRecoveryAttachment(t, writer, 2, [32]byte{4}, [32]byte{5})
	stream := &Stream{current: attachment, sendData: []byte("request"), sendEnd: 7, localTerminal: true,
		terminalSettled: true, terminalGeneration: 1}
	stream.cond = sync.NewCond(&stream.mu)
	stream.startSettledTerminalReplay()
	first, err := ReadStream(reader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadStream(reader)
	if err != nil {
		t.Fatal(err)
	}
	if first.Data == nil || first.Data.AttachmentGeneration != 2 || first.Data.Offset != 0 || string(first.Data.Payload) != "request" {
		t.Fatalf("replayed Data = %+v", first.Data)
	}
	if second.Terminal == nil || second.Terminal.AttachmentGeneration != 2 || second.Terminal.Offset != 7 {
		t.Fatalf("replayed Terminal = %+v", second.Terminal)
	}
	stream.mu.Lock()
	for stream.terminalReplaying && stream.terminal == nil {
		stream.cond.Wait()
	}
	settled := stream.terminalGeneration == 2 && !stream.terminalReplaying && stream.terminal == nil
	stream.mu.Unlock()
	if !settled {
		t.Fatal("settled Terminal replay did not complete")
	}
}

func runTerminalRecoveryJourney(t *testing.T, dropUnacknowledgedData, concurrentHalfClose bool, generation uint64, recoveries uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	clientCarrier, publisherCarrier, fault := newTerminalFaultAdapter(t, dropUnacknowledgedData)
	defer fault.Close()
	freshClient, freshPublisher := net.Pipe()
	defer freshClient.Close()
	defer freshPublisher.Close()
	clientApplication, clientUser := countingHalfClosePair()
	publisherApplication, publisherUser := countingHalfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	connectionContext, exporter, key := [32]byte{1}, [32]byte{2}, [32]byte{3}
	clientInitial := terminalRecoveryAttachment(t, clientCarrier, 1, connectionContext, exporter)
	publisherInitial := terminalRecoveryAttachment(t, publisherCarrier, 1, connectionContext, exporter)
	deadline := time.Now().Add(time.Minute).Unix()
	var clientOpened, publisherOpened atomic.Int32
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientInitial,
		ContinuityKey: key, Authorized: time.Now(), Client: true, Recovery: Recovery{NoNewRecoveryAfter: deadline},
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			clientOpened.Add(1)
			return terminalRecoveryAttachment(t, freshClient, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherInitial,
		ContinuityKey: key, Authorized: time.Now(), Recovery: Recovery{NoNewRecoveryAfter: deadline},
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			publisherOpened.Add(1)
			return terminalRecoveryAttachment(t, freshPublisher, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	request := []byte("request")
	if dropUnacknowledgedData {
		request = bytes.Repeat([]byte("r"), MaximumDataBytes+1)
	}
	limit := uint32(len(request) + 64)
	type result struct {
		name    string
		outcome Outcome
		err     error
	}
	type applicationRead struct {
		data []byte
		err  error
	}
	results := make(chan result, 2)
	go func() {
		outcome, runErr := client.RunBounded(limit, limit)
		results <- result{"client", outcome, runErr}
	}()
	go func() {
		outcome, runErr := publisher.RunBounded(limit, limit)
		results <- result{"publisher", outcome, runErr}
	}()
	publisherRequestResult := make(chan applicationRead, 1)
	go func() {
		data, readErr := io.ReadAll(publisherUser)
		publisherRequestResult <- applicationRead{data, readErr}
	}()
	publisherResponse := make(chan error, 1)
	clientResponse := make([]byte, len("response"))
	clientResponseResult := make(chan error, 1)
	if _, err := clientUser.Write(request); err != nil {
		t.Fatal(err)
	}
	if dropUnacknowledgedData {
		select {
		case <-fault.prefixAcknowledged:
		case <-ctx.Done():
			t.Fatal("fault adapter did not forward acknowledgement for the delivered data prefix")
		}
	}
	if err := clientUser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fault.dropped:
	case <-ctx.Done():
		t.Fatal("fault adapter did not accept the client Terminal")
	}
	if concurrentHalfClose {
		go writeTerminalRecoveryResponse(publisherUser, publisherResponse)
		go func() {
			_, err := io.ReadFull(clientUser, clientResponse)
			clientResponseResult <- err
		}()
	}
	publisherRequest := <-publisherRequestResult
	if publisherRequest.err != nil || !bytes.Equal(publisherRequest.data, request) {
		t.Fatalf("publisher request after recovery = %q, %v; client=%s publisher=%s", publisherRequest.data, publisherRequest.err,
			terminalRecoveryState(client), terminalRecoveryState(publisher))
	}
	if dropUnacknowledgedData && !fault.droppedData {
		t.Fatal("fault adapter did not drop the unacknowledged data suffix")
	}
	if !concurrentHalfClose {
		go writeTerminalRecoveryResponse(publisherUser, publisherResponse)
	}
	if concurrentHalfClose {
		if err := <-clientResponseResult; err != nil || string(clientResponse) != "response" {
			t.Fatalf("concurrent client response = %q, %v", clientResponse, err)
		}
	} else if _, err := io.ReadFull(clientUser, clientResponse); err != nil || string(clientResponse) != "response" {
		t.Fatalf("client response after recovered request EOF = %q, %v", clientResponse, err)
	}
	if err := <-publisherResponse; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var result result
		select {
		case result = <-results:
		case <-time.After(time.Second):
			t.Fatalf("stream completion stalled: client=%s publisher=%s", terminalRecoveryState(client), terminalRecoveryState(publisher))
		}
		if result.err != nil || result.outcome.Generation != generation || result.outcome.Recoveries != recoveries {
			t.Fatalf("%s stream result = %+v, %v; client=%s publisher=%s", result.name, result.outcome, result.err,
				terminalRecoveryState(client), terminalRecoveryState(publisher))
		}
	}
	var afterEOF [1]byte
	if count, err := clientUser.Read(afterEOF[:]); count != 0 || err != io.EOF {
		t.Fatalf("client read after response terminal = %d, %v", count, err)
	}
	if clientApplication.closeInputs.Load() != 1 || publisherApplication.closeInputs.Load() != 1 {
		t.Fatalf("remote Terminal did not produce one directional EOF: client=%d publisher=%d",
			clientApplication.closeInputs.Load(), publisherApplication.closeInputs.Load())
	}
	if clientOpened.Load() != 1 || publisherOpened.Load() != 1 {
		t.Fatalf("replacement attachments = client %d publisher %d", clientOpened.Load(), publisherOpened.Load())
	}
}

func writeTerminalRecoveryResponse(application *halfCloseApplication, result chan<- error) {
	if _, err := application.Write([]byte("response")); err != nil {
		result <- err
		return
	}
	result <- application.CloseWrite()
}

func terminalRecoveryState(stream *Stream) string {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return fmt.Sprintf("base=%d end=%d received=%d acknowledgement=%d/%d terminal-ack=%t/%t receipt-generation=%d local=%t remote=%t terminal-generation=%d current=%d replaying=%t terminal=%v",
		stream.sendBase, stream.sendEnd, stream.recvNext, stream.ackSent, stream.ackPending,
		stream.terminalAckPending, stream.terminalAckSent, stream.terminalAcknowledgedGeneration, stream.localTerminal, stream.remoteTerminal,
		stream.terminalGeneration, stream.currentGenerationLocked(), stream.terminalReplaying, stream.terminal)
}

func terminalRecoveryAttachment(t *testing.T, carrier net.Conn, generation uint64, context, exporter [32]byte) *Attachment {
	t.Helper()
	attachment, err := NewAttachment(carrier, generation, context, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	return attachment
}

type countingHalfCloseApplication struct {
	*halfCloseApplication
	closeInputs atomic.Int32
}

func countingHalfClosePair() (*countingHalfCloseApplication, *halfCloseApplication) {
	application, user := halfClosePair()
	return &countingHalfCloseApplication{halfCloseApplication: application}, user
}

func (application *countingHalfCloseApplication) CloseInput() error {
	application.closeInputs.Add(1)
	return application.halfCloseApplication.CloseInput()
}

type terminalFaultAdapter struct {
	client, publisher      net.Conn
	dropData               bool
	forwardedData          bool
	droppedData            bool
	prefixAcknowledged     chan struct{}
	prefixAcknowledgedOnce sync.Once
	dropped                chan struct{}
	closeOnce              sync.Once
	done                   sync.WaitGroup
}

func newTerminalFaultAdapter(t *testing.T, dropData bool) (net.Conn, net.Conn, *terminalFaultAdapter) {
	t.Helper()
	client, adapterClient := net.Pipe()
	adapterPublisher, publisher := net.Pipe()
	adapter := &terminalFaultAdapter{
		client: adapterClient, publisher: adapterPublisher, dropData: dropData,
		prefixAcknowledged: make(chan struct{}), dropped: make(chan struct{}),
	}
	adapter.done.Add(2)
	go adapter.forward(adapterClient, adapterPublisher, true)
	go adapter.forward(adapterPublisher, adapterClient, false)
	return client, publisher, adapter
}

func (adapter *terminalFaultAdapter) forward(source, destination net.Conn, clientDirection bool) {
	defer adapter.done.Done()
	for {
		record, err := Read(source)
		if err != nil {
			return
		}
		if clientDirection && record.Data != nil {
			if adapter.dropData && adapter.forwardedData {
				adapter.droppedData = true
				continue
			}
			adapter.forwardedData = true
		}
		if clientDirection && record.Terminal != nil {
			close(adapter.dropped)
			adapter.closeOnce.Do(func() {
				_ = adapter.client.Close()
				_ = adapter.publisher.Close()
			})
			return
		}
		if err := Write(destination, record); err != nil {
			return
		}
		if !clientDirection && adapter.dropData && record.Acknowledgement != nil && record.Acknowledgement.Offset >= uint64(MaximumDataBytes) {
			adapter.prefixAcknowledgedOnce.Do(func() { close(adapter.prefixAcknowledged) })
		}
	}
}

func (adapter *terminalFaultAdapter) Close() {
	adapter.closeOnce.Do(func() {
		_ = adapter.client.Close()
		_ = adapter.publisher.Close()
	})
	adapter.done.Wait()
}
