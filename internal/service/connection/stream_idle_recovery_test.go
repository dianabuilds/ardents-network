package connection

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRunBoundedReplaysDataAfterRecoveryWithIdleApplicationInput(t *testing.T) {
	for _, test := range []struct {
		name        string
		lost        bool
		clientSends bool
	}{
		{name: "client-healthy", clientSends: true},
		{name: "client-lost-data", lost: true, clientSends: true},
		{name: "publisher-healthy"},
		{name: "publisher-lost-data", lost: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runIdleInputRecovery(t, test.lost, test.clientSends)
		})
	}
}

type idleInputReadBarrier struct {
	Application
	consumed int
	want     int
	entered  chan struct{}
	once     sync.Once
}

func (application *idleInputReadBarrier) Read(value []byte) (int, error) {
	if application.consumed >= application.want {
		application.once.Do(func() { close(application.entered) })
	}
	read, err := application.Application.Read(value)
	application.consumed += read
	return read, err
}

func runIdleInputRecovery(t *testing.T, lost, clientSends bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	faultConfig := terminalFault{dropData: lost}
	if !clientSends {
		faultConfig = terminalFault{dropPublisherData: lost}
	}
	oldClient, oldPublisher, fault := newTerminalFaultAdapter(t, faultConfig)
	defer fault.Close()
	freshClient, freshPublisher := net.Pipe()
	defer freshClient.Close()
	defer freshPublisher.Close()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	request := bytes.Repeat([]byte("r"), MaximumDataBytes+1)
	senderApplication, senderUser, receiverUser := Application(clientApplication), clientUser, publisherUser
	if !clientSends {
		senderApplication, senderUser, receiverUser = publisherApplication, publisherUser, clientUser
	}
	watched := &idleInputReadBarrier{Application: senderApplication, want: len(request), entered: make(chan struct{})}
	clientInput, publisherInput := Application(clientApplication), Application(publisherApplication)
	if clientSends {
		clientInput = watched
	} else {
		publisherInput = watched
	}
	connectionContext, exporter, key := [32]byte{1}, [32]byte{2}, [32]byte{3}
	deadline := time.Now().Add(time.Minute).Unix()
	recovery := Recovery{CandidateView: [32]byte{4}, IsolationContext: [32]byte{5}, DestinationBinding: [32]byte{6}, RouteProfile: Profile,
		WorkSafetyNotAfter: deadline, WorkSafetyMaximum: deadline, NoNewRecoveryAfter: deadline}
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientInput, NetworkID: [32]byte{7},
		Initial: terminalRecoveryAttachment(t, oldClient, 1, connectionContext, exporter), ContinuityKey: key, Authorized: time.Now(), Client: true,
		Recovery: recovery, OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			return terminalRecoveryAttachment(t, freshClient, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherInput, NetworkID: [32]byte{7},
		Initial: terminalRecoveryAttachment(t, oldPublisher, 1, connectionContext, exporter), ContinuityKey: key, Authorized: time.Now(), Recovery: recovery,
		OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			return terminalRecoveryAttachment(t, freshPublisher, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	clientSend, clientReceive := uint32(len(request)+64), uint32(64)
	publisherSend, publisherReceive := uint32(64), uint32(len(request)+64)
	if !clientSends {
		clientSend, clientReceive = 64, uint32(len(request)+64)
		publisherSend, publisherReceive = uint32(len(request)+64), 64
	}
	go func() { _, runErr := client.RunBounded(clientSend, clientReceive); results <- runErr }()
	go func() { _, runErr := publisher.RunBounded(publisherSend, publisherReceive); results <- runErr }()
	got := make(chan error, 1)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		data := make([]byte, len(request))
		_, readErr := io.ReadFull(receiverUser, data)
		if readErr == nil && !bytes.Equal(data, request) {
			readErr = fmt.Errorf("request differs")
		}
		got <- readErr
	}()
	t.Cleanup(func() {
		cancel()
		_ = clientUser.Close()
		_ = publisherUser.Close()
		select {
		case <-readerDone:
		case <-time.After(time.Second):
			t.Error("Application reader cleanup did not join")
		}
		for range 2 {
			select {
			case <-results:
			case <-time.After(time.Second):
				t.Error("stream cleanup did not join")
			}
		}
	})
	if _, err = senderUser.Write(request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-watched.entered:
	case <-ctx.Done():
		t.Fatal("sender did not enter its next Application read")
	}
	if !lost {
		select {
		case err = <-got:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("healthy request delivery failed")
		}
		return
	}
	select {
	case <-fault.prefixAcknowledged:
	case <-ctx.Done():
		t.Fatal("delivered data prefix was not acknowledged")
	}
	fault.Close()
	select {
	case err = <-got:
		if err != nil {
			t.Fatal(err)
		}
		return
	case <-ctx.Done():
		t.Fatal("recovered Data was not delivered without a new Application input")
	}
}
