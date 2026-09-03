package route

import (
	"errors"
	"net"
	"sync"
	"time"
)

// Attachment is one admitted, fully authenticated User Route byte carrier.
// It owns the Entry cleanup and Route resource release exactly once while the
// caller receives only net.Conn and immutable post-verification Evidence.
type Attachment struct {
	connection net.Conn
	evidence   Evidence

	mu     sync.Mutex
	close  func() error
	finish func(error)
	done   chan struct{}
	result error
}

var _ net.Conn = (*Attachment)(nil)

// Evidence returns a copy of the verified result that binds this carrier to
// the exact Target, publication, and attachment identity. It remains readable
// after Close but never exposes Route peers, State, or credential material.
func (attachment *Attachment) Evidence() (Evidence, error) {
	if attachment == nil {
		return Evidence{}, errors.New("user Route attachment is unavailable")
	}
	attachment.mu.Lock()
	defer attachment.mu.Unlock()
	if attachment.evidence.AuthenticatedTarget == [32]byte{} || attachment.evidence.AttachmentID == [32]byte{} {
		return Evidence{}, errors.New("user Route attachment has no verified evidence")
	}
	result := attachment.evidence
	result.Publication = append([]byte(nil), result.Publication...)
	return result, nil
}

// Close joins concurrent callers on one terminal cleanup. It closes the
// Entry-owned carrier and Route-owned capacity only through its owner-provided
// cleanup function; a later caller observes the same terminal result.
func (attachment *Attachment) Close() error {
	if attachment == nil {
		return errors.New("user Route attachment is unavailable")
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
		return errors.New("user Route attachment is unavailable")
	}
	cleanup, finish := attachment.close, attachment.finish
	attachment.done = make(chan struct{})
	attachment.connection, attachment.close = nil, nil
	attachment.mu.Unlock()
	result := cleanup()
	if finish != nil {
		finish(result)
	} else {
		attachment.publish(result)
	}
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
	if attachment.done != nil {
		close(attachment.done)
	}
	attachment.mu.Unlock()
}

func (attachment *Attachment) carrier() (net.Conn, error) {
	if attachment == nil {
		return nil, errors.New("user Route attachment is unavailable")
	}
	attachment.mu.Lock()
	defer attachment.mu.Unlock()
	if attachment.connection == nil {
		return nil, errors.New("user Route attachment is unavailable")
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
