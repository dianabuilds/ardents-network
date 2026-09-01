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
// receive only the opaque byte carrier and must close it exactly once.
type Attachment struct {
	connection net.Conn

	mu    sync.Mutex
	close func() error
}

var _ net.Conn = (*Attachment)(nil)

// Close releases the authenticated Entry attempt, its caller-owned resource
// reservation, and Route's local selection. A closed Attachment cannot be
// reused for another Service Connection generation.
func (attachment *Attachment) Close() error {
	if attachment == nil {
		return errors.New("native Route attachment is unavailable")
	}
	attachment.mu.Lock()
	if attachment.connection == nil || attachment.close == nil {
		attachment.mu.Unlock()
		return errors.New("native Route attachment is unavailable")
	}
	close := attachment.close
	attachment.connection, attachment.close = nil, nil
	attachment.mu.Unlock()
	return close()
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
	deadline time.Time, admit ResourceAdmission, released func()) (*Attachment, error) {
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
		defer released()
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
