package serviceconn

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	publishCapability = uint32(1)
	connectCapability = uint32(2)
	maximumSessions   = 6
	maximumStream     = uint32(64 << 10)
)

// Credential is one public bounded authorization for an exclusive Instance.
type Credential struct {
	AuthorityPublic [32]byte `json:"authority_public"`
	Target          [32]byte `json:"target"`
	InstancePublic  [32]byte `json:"instance_public"`
	Generation      uint64   `json:"generation"`
	NotBefore       int64    `json:"not_before"`
	NotAfter        int64    `json:"not_after"`
	NetworkID       [32]byte `json:"network_id"`
	Capabilities    uint32   `json:"capabilities"`
	Signature       [64]byte `json:"signature"`
}

// Setup fixes one Endpoint broker generation and its two local principals.
type Setup struct {
	NetworkID               [32]byte
	BrokerID                [32]byte
	AuthorityPublic         ed25519.PublicKey
	IntroductionPublic      ed25519.PublicKey
	ConnectionPrincipal     [32]byte
	AdministrationPrincipal [32]byte
	LastGeneration          uint64
	GenerationStateFile     string
	Clock                   func() time.Time
}

// Request is one role-scoped operation at the Service Connection seam.
type Request struct {
	Action, Surface             string
	Principal, Session, Target  [32]byte
	Credential                  Credential
	InstancePrivate             ed25519.PrivateKey
	IntroductionAcknowledgement []byte
	IntroductionSocket          string
	Publication                 []byte
	Route, Application          io.ReadWriteCloser
	BytesEachDirection          uint32
	At                          time.Time
}

// Result is a bounded product-class outcome with no Route internals.
type Result struct {
	Class                       string   `json:"class"`
	Reason                      string   `json:"reason"`
	Session                     [32]byte `json:"session"`
	Publication                 []byte   `json:"publication"`
	AuthenticatedTarget         [32]byte `json:"authenticated_target"`
	Generation                  uint64   `json:"generation"`
	AcceptedBytes               uint32   `json:"accepted_bytes"`
	ReceivedBytes               uint32   `json:"received_bytes"`
	ConnectionCanary            [32]byte `json:"connection_canary"`
	IntroductionReceipt         [32]byte `json:"introduction_receipt"`
	IntroductionAcknowledgement []byte   `json:"introduction_acknowledgement,omitempty"`
	PrincipalCommitment         [32]byte `json:"principal_commitment"`
	SessionCommitment           [32]byte `json:"session_commitment"`
	GrantSurface                string   `json:"grant_surface"`
	SessionConsumed             bool     `json:"session_consumed"`
	MemoryHighWater             uint64   `json:"memory_high_water"`
	CPUSeconds                  float64  `json:"cpu_seconds"`
	OpenFilesHighWater          uint32   `json:"open_files_high_water"`
	GoroutinesHighWater         uint32   `json:"goroutines_high_water"`
	ActiveSessions              uint32   `json:"active_sessions"`
	TimerHighWater              uint32   `json:"timer_high_water"`
	QueueHighWater              uint32   `json:"queue_high_water"`
	TempEntries                 uint32   `json:"temp_entries"`
}

// Endpoint owns one broker generation's sessions and current publication.
type Endpoint struct {
	mu                  sync.Mutex
	network, broker     [32]byte
	authority           [32]byte
	introduction        [32]byte
	connectionPrincipal [32]byte
	adminPrincipal      [32]byte
	sessions            map[[32]byte]localSession
	current             *currentPublication
	lastGeneration      uint64
	generationStateFile string
	clock               func() time.Time
}

// New creates one finite Endpoint-local admission and publication boundary.
func New(input Setup) (*Endpoint, error) {
	if input.NetworkID == [32]byte{} || input.BrokerID == [32]byte{} ||
		len(input.AuthorityPublic) != ed25519.PublicKeySize || len(input.IntroductionPublic) != ed25519.PublicKeySize ||
		input.ConnectionPrincipal == [32]byte{} {
		return nil, errors.New("endpoint setup is incomplete")
	}
	var authority [32]byte
	copy(authority[:], input.AuthorityPublic)
	var introduction [32]byte
	copy(introduction[:], input.IntroductionPublic)
	lastGeneration, err := readGeneration(input.GenerationStateFile, input.LastGeneration)
	if err != nil {
		return nil, err
	}
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Endpoint{network: input.NetworkID, broker: input.BrokerID, authority: authority,
		introduction:        introduction,
		connectionPrincipal: input.ConnectionPrincipal, adminPrincipal: input.AdministrationPrincipal,
		sessions: make(map[[32]byte]localSession, maximumSessions), lastGeneration: lastGeneration,
		generationStateFile: input.GenerationStateFile, clock: clock}, nil
}

// Do executes one admitted operation and returns only an R-002 product class.
func (endpoint *Endpoint) Do(ctx context.Context, input Request) (Result, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	monitor := startResourceMonitor()
	var result Result
	var err error
	switch input.Action {
	case "admit":
		result, err = endpoint.admit(input)
	case "publish":
		result, err = endpoint.publish(ctx, input)
	case "unpublish":
		result, err = endpoint.unpublish(input)
	case "connect":
		result, err = endpoint.connect(ctx, input)
	case "accept":
		result, err = endpoint.accept(ctx, input)
	default:
		return denied("local operation is not permitted")
	}
	endpoint.observe(&result, input, monitor.stop())
	return result, err
}

func denied(reason string) (Result, error) {
	return failed("local authorization or policy denial", reason, errors.New(reason))
}

func failed(class, reason string, err error) (Result, error) {
	return Result{Class: class, Reason: reason}, err
}
