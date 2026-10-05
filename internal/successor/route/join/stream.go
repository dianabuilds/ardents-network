package join

import (
	"context"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"net"
	"sync"
	"time"
)

// Joined is one protected physical stream, not an authenticated Service
// Connection. The original acquisition remains held until Close joins all
// framing and lower-channel borrowers.
type Joined struct {
	acquisition           *JoinAcquisition
	session               *framing.Session
	lane                  *framing.Lane
	parent                interface{ CloseParent() error }
	once                  sync.Once
	result                error
	retiring, watcherDone chan struct{}
	stopOnce              sync.Once
	terminal              error
}

func (j *Joined) Read(body []byte) (int, error) {
	if err := j.acquisition.current(); err != nil {
		return 0, err
	}
	n, err := j.lane.Read(body)
	if current := j.acquisition.current(); current != nil {
		return 0, errors.Join(err, current)
	}
	return n, err
}

func (j *Joined) Write(body []byte) (int, error) {
	if err := j.acquisition.current(); err != nil {
		return 0, err
	}
	n, err := j.lane.Write(body)
	return n, errors.Join(err, j.acquisition.current())
}

func (j *Joined) CloseWrite() error {
	if err := j.acquisition.current(); err != nil {
		return err
	}
	return errors.Join(j.lane.CloseWrite(), j.acquisition.current())
}

func (j *Joined) Close() error                       { return j.acquisition.Close() }
func (j *Joined) LocalAddr() net.Addr                { return j.lane.LocalAddr() }
func (j *Joined) RemoteAddr() net.Addr               { return j.lane.RemoteAddr() }
func (j *Joined) SetDeadline(t time.Time) error      { return j.lane.SetDeadline(t) }
func (j *Joined) SetReadDeadline(t time.Time) error  { return j.lane.SetReadDeadline(t) }
func (j *Joined) SetWriteDeadline(t time.Time) error { return j.lane.SetWriteDeadline(t) }

// Synchronous effect seal contains no I/O or waits. The watcher then interrupts
// and joins the already selected writer; scheduling it is not revocation.
func (j *Joined) seal() {
	j.lane.Seal()
}

func (j *Joined) closePhysical() error {
	j.once.Do(func() {
		j.stopOnce.Do(func() { close(j.retiring) })
		<-j.watcherDone
		if j.terminal == nil {
			// Sending local CLOSE does not prove the pair's opposite writer
			// joined. Read its authenticated terminal before retiring the lower
			// lane; otherwise that retirement races the peer's pending CLOSE.
			j.terminal = j.session.WaitTerminal(time.Now().Add(time.Second))
		}
		j.result = errors.Join(j.terminal, j.session.Close(), j.parent.CloseParent())
	})
	return j.result
}

func (j *Joined) watch(caller context.Context) {
	defer close(j.watcherDone)
	select {
	case <-j.retiring:
	case <-j.acquisition.ctx.Done():
	case <-caller.Done():
	}
	// Seal writes and interrupt/join any selected writer before terminal output.
	j.terminal = j.lane.Close()
}

var _ net.Conn = (*Joined)(nil)
