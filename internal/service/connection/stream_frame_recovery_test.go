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

// The receiver accepts the complete Data frame before its acknowledgement is
// lost. Continuity must carry that accepted offset across recovery without
// presenting the same Application bytes again.
func TestRunBoundedRecoversLostDataAcknowledgementWithoutReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldClient, oldPublisher, fault := newTerminalFaultAdapter(t, terminalFault{dropDataAcknowledgement: true})
	defer fault.Close()
	freshClient, freshPublisher := net.Pipe()
	defer freshClient.Close()
	defer freshPublisher.Close()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	request, response := []byte("accepted request"), []byte("response")
	connectionContext, exporter, key := [32]byte{1}, [32]byte{2}, [32]byte{3}
	deadline := time.Now().Add(time.Minute).Unix()
	recovery := Recovery{NoNewRecoveryAfter: deadline}
	var clientOpened, publisherOpened atomic.Int32
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication,
		Initial:       terminalRecoveryAttachment(t, oldClient, 1, connectionContext, exporter),
		ContinuityKey: key, Authorized: time.Now(), Client: true, Recovery: recovery,
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			clientOpened.Add(1)
			return terminalRecoveryAttachment(t, freshClient, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication,
		Initial:       terminalRecoveryAttachment(t, oldPublisher, 1, connectionContext, exporter),
		ContinuityKey: key, Authorized: time.Now(), Recovery: recovery,
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			publisherOpened.Add(1)
			return terminalRecoveryAttachment(t, freshPublisher, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		name    string
		outcome Outcome
		err     error
	}
	results := make(chan result, 2)
	go func() {
		outcome, runErr := client.RunBounded(64, 64)
		results <- result{"client", outcome, runErr}
	}()
	go func() {
		outcome, runErr := publisher.RunBounded(64, 64)
		results <- result{"publisher", outcome, runErr}
	}()

	accepted := make(chan struct{})
	publisherRead := make(chan error, 1)
	go func() {
		got := make([]byte, len(request))
		_, readErr := io.ReadFull(publisherUser, got)
		if readErr == nil && !bytes.Equal(got, request) {
			readErr = errors.New("accepted request differs")
		}
		if readErr == nil {
			close(accepted)
			var extra [1]byte
			if count, terminalErr := publisherUser.Read(extra[:]); count != 0 || !errors.Is(terminalErr, io.EOF) {
				readErr = errors.New("accepted request was replayed or lacked directional EOF")
			}
		}
		publisherRead <- readErr
	}()
	if _, err := clientUser.Write(request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-accepted:
	case <-ctx.Done():
		t.Fatal("publisher did not accept the Data frame")
	}
	select {
	case <-fault.dropped:
	case <-ctx.Done():
		t.Fatal("fault adapter did not drop the accepted Data acknowledgement")
	}
	if err := clientUser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if _, err := publisherUser.Write(response); err != nil {
		t.Fatal(err)
	}
	if err := publisherUser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	gotResponse, err := io.ReadAll(clientUser)
	if err != nil || !bytes.Equal(gotResponse, response) {
		t.Fatalf("recovered response = %q, %v", gotResponse, err)
	}
	if err := <-publisherRead; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case result := <-results:
			if result.err != nil || result.outcome.Generation != 2 || result.outcome.Recoveries != 1 {
				t.Fatalf("%s stream = %+v, %v; client=%s publisher=%s", result.name, result.outcome, result.err,
					terminalRecoveryState(client), terminalRecoveryState(publisher))
			}
		case <-ctx.Done():
			t.Fatalf("lost-ack recovery stalled: client=%s publisher=%s", terminalRecoveryState(client), terminalRecoveryState(publisher))
		}
	}
	if clientOpened.Load() != 1 || publisherOpened.Load() != 1 {
		t.Fatalf("replacement Attachments = client %d publisher %d", clientOpened.Load(), publisherOpened.Load())
	}
}

func TestPendingTerminalReceiptUsesExactRemoteTerminalOffset(t *testing.T) {
	attachment := &Attachment{generation: 2}
	stream := &Stream{ackPending: 0, ackSent: 0, terminalAckPending: true,
		terminalAckPendingGeneration: 2, terminalAckOffset: 8}
	stream.mu.Lock()
	offset, already, terminal, confirmation := stream.pendingAcknowledgementLocked(attachment)
	stream.mu.Unlock()
	if offset != 8 || already != 0 || !terminal || confirmation {
		t.Fatalf("pending Terminal receipt = offset %d already %d terminal %t confirmation %t", offset, already, terminal, confirmation)
	}
}

type terminalFaultAdapter struct {
	client, publisher         net.Conn
	dropData                  bool
	dropPublisherData         bool
	dropDataAcknowledgement   bool
	dropClientTerminal        bool
	dropPublisherConfirmation bool
	dropPublisherReceipt      bool
	forwardedData             bool
	forwardedPublisherData    bool
	droppedData               bool
	prefixAcknowledged        chan struct{}
	prefixAcknowledgedOnce    sync.Once
	terminalReceipt           chan struct{}
	terminalReceiptOnce       sync.Once
	dropped                   chan struct{}
	closeOnce                 sync.Once
	done                      sync.WaitGroup
}

type terminalFault struct {
	dropData                  bool
	dropPublisherData         bool
	dropDataAcknowledgement   bool
	dropClientTerminal        bool
	dropPublisherConfirmation bool
	dropPublisherReceipt      bool
}

func newTerminalFaultAdapter(t *testing.T, fault terminalFault) (net.Conn, net.Conn, *terminalFaultAdapter) {
	t.Helper()
	client, adapterClient := net.Pipe()
	adapterPublisher, publisher := net.Pipe()
	adapter := &terminalFaultAdapter{
		client: adapterClient, publisher: adapterPublisher, dropData: fault.dropData, dropPublisherData: fault.dropPublisherData,
		dropDataAcknowledgement: fault.dropDataAcknowledgement, dropClientTerminal: fault.dropClientTerminal,
		dropPublisherReceipt: fault.dropPublisherReceipt, dropPublisherConfirmation: fault.dropPublisherConfirmation,
		prefixAcknowledged: make(chan struct{}), terminalReceipt: make(chan struct{}), dropped: make(chan struct{}),
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
		if !clientDirection && record.Data != nil {
			if adapter.dropPublisherData && adapter.forwardedPublisherData {
				adapter.droppedData = true
				continue
			}
			adapter.forwardedPublisherData = true
		}
		if !clientDirection && record.Acknowledgement != nil && !record.Acknowledgement.Terminal && adapter.dropDataAcknowledgement {
			adapter.dropAndClose()
			return
		}
		if clientDirection && record.Terminal != nil && adapter.dropClientTerminal {
			adapter.dropAndClose()
			return
		}
		if !clientDirection && record.Acknowledgement != nil && record.Acknowledgement.Terminal &&
			!record.Acknowledgement.TerminalConfirmation && adapter.dropPublisherReceipt {
			adapter.dropAndClose()
			return
		}
		if !clientDirection && record.Acknowledgement != nil && record.Acknowledgement.TerminalConfirmation && adapter.dropPublisherConfirmation {
			continue
		}
		if err := Write(destination, record); err != nil {
			return
		}
		if clientDirection && record.Acknowledgement != nil && record.Acknowledgement.Terminal &&
			!record.Acknowledgement.TerminalConfirmation {
			adapter.terminalReceiptOnce.Do(func() { close(adapter.terminalReceipt) })
		}
		if record.Acknowledgement != nil && record.Acknowledgement.Offset >= uint64(MaximumDataBytes) &&
			((adapter.dropData && !clientDirection) || (adapter.dropPublisherData && clientDirection)) {
			adapter.prefixAcknowledgedOnce.Do(func() { close(adapter.prefixAcknowledged) })
		}
	}
}

func (adapter *terminalFaultAdapter) dropAndClose() {
	close(adapter.dropped)
	adapter.closeOnce.Do(func() {
		_ = adapter.client.Close()
		_ = adapter.publisher.Close()
	})
}

func (adapter *terminalFaultAdapter) Close() {
	adapter.closeOnce.Do(func() {
		_ = adapter.client.Close()
		_ = adapter.publisher.Close()
	})
	adapter.done.Wait()
}
