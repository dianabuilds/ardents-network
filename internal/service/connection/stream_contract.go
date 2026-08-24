package connection

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	logicalQueueLimit = 256 << 10
	proposalLimit     = 3
	recoveryLimit     = 15 * time.Second
)

var (
	// ErrActiveViolation means a peer contradicted the authenticated logical
	// stream, so the connection must fail closed rather than try another Route.
	ErrActiveViolation   = errors.New("detected Service Connection integrity violation")
	errRecoveryTerminal  = errors.New("service Connection recovery terminated")
	errWorkSafetyExpired = errors.New("authenticated Work Safety expired")
)

// Attachment is one already-authenticated Route byte carrier. TLS and Route
// selection stay outside this package; the lifecycle receives only the exact
// immutable facts that its native records must bind.
type Attachment struct {
	carrier                     io.ReadWriteCloser
	generation                  uint64
	context, exporterCommitment [32]byte
	close                       func()
}

// NewAttachment admits one authenticated carrier for a fixed attachment
// generation. close may additionally release Route-local resources.
func NewAttachment(carrier io.ReadWriteCloser, generation uint64, connectionContext,
	exporterCommitment [32]byte, close func()) (*Attachment, error) {
	if carrier == nil || generation == 0 || connectionContext == [32]byte{} || exporterCommitment == [32]byte{} {
		return nil, errors.New("authenticated connection attachment is incomplete")
	}
	if close == nil {
		close = func() { _ = carrier.Close() }
	}
	return &Attachment{carrier: carrier, generation: generation, context: connectionContext,
		exporterCommitment: exporterCommitment, close: close}, nil
}

func (attachment *Attachment) closeCarrier() {
	if attachment != nil && attachment.close != nil {
		attachment.close()
	}
}

// AttachmentOpener supplies another already-authenticated, exact-context
// Attachment. It cannot mutate the Stream's identity or recovery contract.
type AttachmentOpener func(context.Context, Recovery) (*Attachment, error)

// StreamConfig binds one logical application stream to one authenticated
// Attachment and, optionally, a finite replacement source.
type StreamConfig struct {
	Context        context.Context
	Application    io.ReadWriteCloser
	NetworkID      [32]byte
	Recovery       Recovery
	OpenAttachment AttachmentOpener
	Initial        *Attachment
	ContinuityKey  [32]byte
	Authorized     time.Time
	Client         bool
	NameBinding    DestinationBinding
	NameUpdates    <-chan DestinationBinding
	Resources      func(string, int) uint32
}

// Outcome is the terminal native logical-stream evidence. Product outcome
// classification belongs to application/broker composition, not this package.
type Outcome struct {
	Accepted, Acknowledged, Received uint32
	QueueHigh                        uint32
	Generation                       uint64
	Recoveries                       uint32
	ContinuityCommitment             [32]byte
}

// Stream owns ordered/replayed logical byte state, attachment replacement and
// the native terminal outcome. It has no local admission or Application IPC
// authorization authority.
type Stream struct {
	ctx         context.Context
	application io.ReadWriteCloser
	networkID   [32]byte
	recovery    Recovery
	opener      AttachmentOpener
	continuity  [32]byte
	client      bool
	authorized  time.Time
	started     time.Time
	resources   func(string, int) uint32
	nameBinding DestinationBinding
	nameUpdates <-chan DestinationBinding
	done        chan struct{}

	mu       sync.Mutex
	cond     *sync.Cond
	writerMu sync.Mutex
	current  *Attachment

	recovering   bool
	established  bool
	terminal     error
	recoveries   uint32
	proposals    int
	episodeEnd   time.Time
	lastProgress time.Time
	ackSignal    chan struct{}

	sendBase, sendEnd, sendNext uint64
	sendData                    []byte
	recvNext, recentAt          uint64
	recent                      []byte
	pending                     []receivedRange
	ackPending, ackSent         uint64
	queueMax                    uint32
	localTerminal               bool
	remoteTerminal              bool
	terminalGeneration          uint64
}

type receivedRange struct {
	offset uint64
	data   []byte
}

// NewStream creates one native logical stream over an already authenticated
// first Attachment. The caller can enable recovery only through OpenAttachment.
func NewStream(input StreamConfig) (*Stream, error) {
	if input.Context == nil || input.Application == nil || input.Initial == nil || input.ContinuityKey == [32]byte{} ||
		input.Authorized.IsZero() {
		return nil, errors.New("native stream setup is incomplete")
	}
	if input.Resources == nil {
		input.Resources = func(string, int) uint32 { return 0 }
	}
	now := time.Now()
	stream := &Stream{ctx: input.Context, application: input.Application, networkID: input.NetworkID, recovery: input.Recovery,
		opener: input.OpenAttachment, continuity: input.ContinuityKey, client: input.Client,
		authorized: input.Authorized, started: now, lastProgress: now, resources: input.Resources,
		nameBinding: input.NameBinding, nameUpdates: input.NameUpdates, done: make(chan struct{}),
		current: input.Initial, ackSignal: make(chan struct{}, 1)}
	stream.cond = sync.NewCond(&stream.mu)
	return stream, nil
}

func (stream *Stream) authorizationTime() time.Time {
	return stream.authorized.Add(time.Since(stream.started))
}

func (stream *Stream) continuityCommitment() [32]byte {
	return sha256.Sum256(append([]byte("ardents-service-connection-continuity-commitment-v1\x00"), stream.continuity[:]...))
}
