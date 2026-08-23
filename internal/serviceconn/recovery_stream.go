package serviceconn

import (
	"context"
	"crypto"
	"crypto/sha256"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	logicalQueueLimit = 256 << 10
	proposalLimit     = 3
	recoveryLimit     = 15 * time.Second
)

var errRecoveryTerminal = errors.New("service Connection recovery terminated")
var errWorkSafetyExpired = errors.New("authenticated Work Safety expired")

type recoveryStream struct {
	ctx         context.Context
	application io.ReadWriteCloser
	credential  Credential
	binding     Recovery
	private     crypto.Signer
	client      bool
	opener      func(context.Context, Recovery) (net.Conn, error)
	continuity  [32]byte
	context     [32]byte
	ackSignal   chan struct{}
	authorized  time.Time
	started     time.Time
	resources   func(string, int) uint32
	nameBinding DestinationBinding
	nameUpdates <-chan DestinationBinding
	done        chan struct{}

	mu           sync.Mutex
	cond         *sync.Cond
	writerMu     sync.Mutex
	current      *securedAttachment
	recovering   bool
	terminal     error
	recoveries   uint32
	proposals    int
	episodeEnd   time.Time
	lastProgress time.Time

	sendBase   uint64
	sendEnd    uint64
	sendNext   uint64
	sendData   []byte
	recvNext   uint64
	recentAt   uint64
	recent     []byte
	pending    []receivedRange
	ackPending uint64
	ackSent    uint64
	queueMax   uint32
}

type receivedRange struct {
	offset uint64
	data   []byte
}

type recoveryOutcome struct {
	accepted, acknowledged, received uint32
	queueHigh                        uint32
	generation                       uint64
	recoveries                       uint32
	continuity                       [32]byte
}

func newRecoveryStream(ctx context.Context, application io.ReadWriteCloser, credential Credential,
	binding Recovery, private crypto.Signer, client bool,
	opener func(context.Context, Recovery) (net.Conn, error), initial *securedAttachment,
	continuity [32]byte, authorized time.Time, nameBinding DestinationBinding,
	nameUpdates <-chan DestinationBinding,
	resources func(string, int) uint32) *recoveryStream {
	now := time.Now()
	stream := &recoveryStream{ctx: ctx, application: application, credential: credential, binding: binding,
		private: private, client: client, opener: opener, current: initial, continuity: continuity,
		context:   initial.context,
		ackSignal: make(chan struct{}, 1), authorized: authorized, started: now, lastProgress: now,
		nameBinding: nameBinding, nameUpdates: nameUpdates, done: make(chan struct{}), resources: resources}
	stream.cond = sync.NewCond(&stream.mu)
	return stream
}

func (stream *recoveryStream) authorizationTime() time.Time {
	return stream.authorized.Add(time.Since(stream.started))
}

func (stream *recoveryStream) run(sendCount, receiveCount uint32) (recoveryOutcome, error) {
	defer close(stream.done)
	stream.watchNameOrigin()
	stop := context.AfterFunc(stream.ctx, func() { stream.fail(stream.ctx.Err()) })
	defer stop()
	if stream.binding.WorkSafetyNotAfter != 0 {
		remaining := time.Unix(stream.binding.WorkSafetyNotAfter, 0).Sub(stream.authorizationTime())
		releaseTimer := acquireResource(stream.resources, "timer")
		safetyTimer := time.AfterFunc(remaining, func() { stream.fail(errWorkSafetyExpired) })
		defer func() {
			safetyTimer.Stop()
			releaseTimer()
		}()
	}
	defer stream.close()
	dataResults := make(chan error, 2)
	ackResult := make(chan error, 1)
	go func() { dataResults <- stream.sendApplication(uint64(sendCount)) }()
	go func() { dataResults <- stream.receiveApplication(uint64(receiveCount), uint64(sendCount)) }()
	go func() { ackResult <- stream.sendAcknowledgements(uint64(receiveCount)) }()
	first := <-dataResults
	if errors.Is(first, errActiveViolation) || errors.Is(first, errRecoveryTerminal) {
		stream.fail(first)
	}
	second := <-dataResults
	dataErr := errors.Join(first, second)
	if dataErr != nil {
		stream.fail(dataErr)
	}
	err := errors.Join(dataErr, <-ackResult)
	stream.mu.Lock()
	if err == nil {
		err = stream.terminal
	}
	outcome := recoveryOutcome{accepted: uint32(stream.sendEnd), acknowledged: uint32(stream.sendBase),
		received:  uint32(stream.recvNext),
		queueHigh: stream.queueMax, generation: stream.currentGenerationLocked(), recoveries: stream.recoveries,
		continuity: sha256.Sum256(append([]byte("ardents-h3-continuity-commitment-v1\x00"), stream.continuity[:]...))}
	stream.mu.Unlock()
	return outcome, err
}

func (stream *recoveryStream) attachment() (*securedAttachment, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for stream.recovering && stream.terminal == nil {
		stream.cond.Wait()
	}
	if stream.terminal != nil {
		return nil, stream.terminal
	}
	return stream.current, nil
}

func (stream *recoveryStream) fail(err error) {
	if err == nil {
		return
	}
	stream.mu.Lock()
	if stream.terminal == nil {
		stream.terminal = err
		if stream.current != nil {
			stream.current.close()
		}
		if deadline, ok := stream.application.(interface{ SetDeadline(time.Time) error }); ok {
			_ = deadline.SetDeadline(time.Now())
		} else {
			_ = stream.application.Close()
		}
	}
	stream.recovering = false
	stream.cond.Broadcast()
	stream.mu.Unlock()
	select {
	case stream.ackSignal <- struct{}{}:
	default:
	}
}

func (stream *recoveryStream) close() {
	stream.mu.Lock()
	if stream.current != nil {
		stream.current.close()
	}
	stream.mu.Unlock()
	erase(stream.continuity[:])
}

func (stream *recoveryStream) currentGenerationLocked() uint64 {
	if stream.current == nil {
		return 0
	}
	return stream.current.generation
}
