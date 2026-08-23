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
	Connection net.Conn

	mu    sync.Mutex
	close func() error
}

// Close releases the authenticated Entry attempt, its caller-owned resource
// reservation, and Route's local selection. A closed Attachment cannot be
// reused for another Service Connection generation.
func (attachment *Attachment) Close() error {
	if attachment == nil {
		return errors.New("native Route attachment is unavailable")
	}
	attachment.mu.Lock()
	if attachment.Connection == nil || attachment.close == nil {
		attachment.mu.Unlock()
		return errors.New("native Route attachment is unavailable")
	}
	close := attachment.close
	attachment.Connection, attachment.close = nil, nil
	attachment.mu.Unlock()
	return close()
}

func openNativeAttachment(ctx context.Context, source EntryAcquirer, network, digest [32]byte, epoch uint64,
	identifier [32]byte, deadline time.Time, admit ResourceAdmission, released func()) (*Attachment, error) {
	if source == nil || network == [32]byte{} || digest == [32]byte{} || epoch == 0 || identifier == [32]byte{} ||
		deadline.IsZero() || !time.Now().Before(deadline) || admit == nil {
		return nil, errors.New("native Route attachment request is invalid")
	}
	release, err := admit(ctx)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, errors.New("native Route resource admission has no release")
	}
	connection, closeAttempt, err := OpenEntryAttachment(ctx, source, EntryAttachmentRequest{
		NetworkID: network, Digest: digest, Epoch: epoch, AttachmentID: identifier, Deadline: deadline,
	})
	if err != nil {
		return nil, errors.Join(err, release())
	}
	return &Attachment{Connection: connection, close: func() error {
		defer released()
		return errors.Join(connection.Close(), closeAttempt(), release())
	}}, nil
}
