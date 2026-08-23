package route

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"time"
)

// NativeAttachmentRequest contains the endpoint-selected facts needed to open
// one native User-to-Initiator attachment. Admit is the caller-owned resource
// decision; Route does not select a Node resource profile.
type NativeAttachmentRequest struct {
	Entry             EntryAcquirer
	NetworkID, Digest [32]byte
	Epoch             uint64
	Deadline          time.Time
	Admit             func(context.Context) error
}

// NativeAttachment is one admitted native Entry connection. Its identifier is
// created by Route and is never a command or caller-selected plan field.
type NativeAttachment struct {
	ID         [32]byte
	Connection net.Conn
	close      func() error
}

// Close releases the Entry attempt state and its authenticated connection.
func (attachment *NativeAttachment) Close() error {
	if attachment == nil || attachment.Connection == nil || attachment.close == nil {
		return errors.New("native Route attachment is unavailable")
	}
	err := attachment.close()
	attachment.Connection, attachment.close = nil, nil
	return err
}

// OpenNativeAttachment obtains resource admission before allocating a fresh
// Entry attempt. A refusal never dials an Entry candidate or consumes an Invite.
func OpenNativeAttachment(ctx context.Context, input NativeAttachmentRequest) (*NativeAttachment, error) {
	if input.Entry == nil || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.Epoch == 0 ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) || input.Admit == nil {
		return nil, errors.New("native Route attachment request is invalid")
	}
	if err := input.Admit(ctx); err != nil {
		return nil, err
	}
	var identifier [32]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return nil, err
	}
	connection, closeAttempt, err := OpenEntryAttachment(ctx, input.Entry, EntryAttachmentRequest{
		NetworkID: input.NetworkID, Digest: input.Digest, Epoch: input.Epoch, AttachmentID: identifier,
		Deadline: input.Deadline,
	})
	if err != nil {
		return nil, err
	}
	return &NativeAttachment{ID: identifier, Connection: connection, close: func() error {
		return errors.Join(connection.Close(), closeAttempt())
	}}, nil
}
