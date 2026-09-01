package route

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Attachment is one admitted native Entry connection. Route creates and owns
// its identifier, selection, resource reservation, and Entry cleanup; callers
// receive only the opaque byte carrier. Cleanup executes once, while every
// concurrent or later Close joins its terminal result.
type Attachment struct {
	connection net.Conn

	mu     sync.Mutex
	close  func() error
	finish func(error)
	done   chan struct{}
	result error
}

var _ net.Conn = (*Attachment)(nil)

// Close attempts the authenticated Entry, caller-owned resource, and local
// Route-selection cleanup as one owned operation. Concurrent and later calls
// join the same terminal result; a nil result proves release happened exactly
// once. A closed Attachment cannot be reused for another Service Connection
// generation.
func (attachment *Attachment) Close() error {
	if attachment == nil {
		return errors.New("native Route attachment is unavailable")
	}
	attachment.mu.Lock()
	if attachment.done != nil {
		done := attachment.done
		attachment.mu.Unlock()
		<-done
		attachment.mu.Lock()
		result := attachment.result
		attachment.mu.Unlock()
		return result
	}
	if attachment.connection == nil || attachment.close == nil {
		attachment.mu.Unlock()
		return errors.New("native Route attachment is unavailable")
	}
	cleanup := attachment.close
	finish := attachment.finish
	done := make(chan struct{})
	attachment.done = done
	attachment.connection, attachment.close = nil, nil
	attachment.mu.Unlock()
	result := cleanup()
	if finish != nil {
		finish(result)
		return result
	}
	attachment.publish(result)
	return result
}

func (attachment *Attachment) bindCompletion(finish func(error)) {
	attachment.mu.Lock()
	attachment.finish = finish
	attachment.mu.Unlock()
}

func (attachment *Attachment) publish(result error) {
	attachment.mu.Lock()
	attachment.result = result
	close(attachment.done)
	attachment.mu.Unlock()
}

func (attachment *Attachment) carrier() (net.Conn, error) {
	if attachment == nil {
		return nil, errors.New("native Route attachment is unavailable")
	}
	attachment.mu.Lock()
	defer attachment.mu.Unlock()
	if attachment.connection == nil {
		return nil, errors.New("native Route attachment is unavailable")
	}
	return attachment.connection, nil
}

func (attachment *Attachment) Read(destination []byte) (int, error) {
	connection, err := attachment.carrier()
	if err != nil {
		return 0, err
	}
	return connection.Read(destination)
}

func (attachment *Attachment) Write(source []byte) (int, error) {
	connection, err := attachment.carrier()
	if err != nil {
		return 0, err
	}
	return connection.Write(source)
}

func (attachment *Attachment) LocalAddr() net.Addr {
	connection, err := attachment.carrier()
	if err != nil {
		return nil
	}
	return connection.LocalAddr()
}

func (attachment *Attachment) RemoteAddr() net.Addr {
	connection, err := attachment.carrier()
	if err != nil {
		return nil
	}
	return connection.RemoteAddr()
}

func (attachment *Attachment) SetDeadline(deadline time.Time) error {
	connection, err := attachment.carrier()
	if err != nil {
		return err
	}
	return connection.SetDeadline(deadline)
}

func (attachment *Attachment) SetReadDeadline(deadline time.Time) error {
	connection, err := attachment.carrier()
	if err != nil {
		return err
	}
	return connection.SetReadDeadline(deadline)
}

func (attachment *Attachment) SetWriteDeadline(deadline time.Time) error {
	connection, err := attachment.carrier()
	if err != nil {
		return err
	}
	return connection.SetWriteDeadline(deadline)
}

func openNativeAttachment(ctx context.Context, source EntryAcquirer, selected plan, identifier [32]byte,
	deadline time.Time, admit ResourceAdmission) (*Attachment, error) {
	if source == nil || selected.networkID == [32]byte{} || selected.digest == [32]byte{} || selected.epoch == 0 || identifier == [32]byte{} ||
		deadline.IsZero() || !time.Now().Before(deadline) || admit == nil {
		return nil, errors.New("native Route attachment request is invalid")
	}
	setup, err := selected.relaySetup(identifier, deadline)
	if err != nil {
		return nil, err
	}
	release, err := admit(ctx)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, errors.New("native Route resource admission has no release")
	}
	connection, closeAttempt, err := OpenEntryAttachment(ctx, source, EntryAttachmentRequest{
		NetworkID: selected.networkID, Digest: selected.digest, Epoch: selected.epoch, AttachmentID: identifier, Deadline: deadline,
	})
	if err != nil {
		return nil, errors.Join(err, release())
	}
	if err := WriteRelaySetup(connection, setup); err != nil {
		return nil, errors.Join(err, connection.Close(), closeAttempt(), release())
	}
	ready, err := ReadRelayReady(connection)
	if err != nil {
		return nil, errors.Join(err, connection.Close(), closeAttempt(), release())
	}
	if err := setup.VerifyRelayReady(ready); err != nil {
		return nil, errors.Join(err, connection.Close(), closeAttempt(), release())
	}
	return &Attachment{connection: connection, close: func() error {
		return errors.Join(connection.Close(), closeAttempt(), release())
	}}, nil
}

func (selected plan) relaySetup(identifier [32]byte, deadline time.Time) (RelaySetup, error) {
	var initiator, rendezvous position
	for _, value := range selected.positions {
		switch value.role {
		case "initiator":
			initiator = value
		case "rendezvous":
			rendezvous = value
		}
	}
	setup := RelaySetup{NetworkID: selected.networkID, Digest: selected.digest, AttachmentID: identifier, Epoch: selected.epoch,
		TransitRole: InitiatorRole, NextRole: RendezvousRole, TransitNodeID: initiator.nodeID, NextNodeID: rendezvous.nodeID,
		NextNodePublicKey: rendezvous.publicKey, NotAfter: deadline.UTC().Truncate(time.Second)}
	if err := validRelaySetup(setup); err != nil {
		return RelaySetup{}, err
	}
	return setup, nil
}
