package serviceconn

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	publishCapability = uint32(1)
	connectCapability = uint32(2)
	maximumSessions   = 6
	maximumStream     = uint32(4 << 20)
)

// Recovery fixes connection values and constrains one fresh Route Attachment.
type Recovery struct {
	Generation         uint64
	Deadline           time.Time
	NetworkID          [32]byte
	CandidateView      [32]byte
	IsolationContext   [32]byte
	DestinationBinding [32]byte
	RouteProfile       string
	Role               string
	WorkSafetyNotAfter int64
	WorkSafetyMaximum  int64
	NoNewRecoveryAfter int64
}

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
	Resources               func(string, int) uint32
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
	Route                       net.Conn
	Application                 io.ReadWriteCloser
	OpenAttachment              func(context.Context, Recovery) (net.Conn, error)
	RecoveryBinding             Recovery
	BytesEachDirection          uint32
	SendBytes, ReceiveBytes     uint32
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
	RouteGeneration             uint64   `json:"route_generation"`
	RecoveryCount               uint32   `json:"recovery_count"`
	ContinuityCommitment        [32]byte `json:"continuity_commitment"`
	AcceptedBytes               uint32   `json:"accepted_bytes"`
	AcknowledgedBytes           uint32   `json:"acknowledged_bytes"`
	ReceivedBytes               uint32   `json:"received_bytes"`
	ConnectionCanary            [32]byte `json:"connection_canary"`
	IntroductionReceipt         [32]byte `json:"introduction_receipt"`
	IntroductionAcknowledgement []byte   `json:"introduction_acknowledgement,omitempty"`
	PrincipalCommitment         [32]byte `json:"principal_commitment"`
	SessionCommitment           [32]byte `json:"session_commitment"`
	GrantSurface                string   `json:"grant_surface"`
	SessionConsumed             bool     `json:"session_consumed"`
	BrokerCommitment            [32]byte `json:"broker_commitment"`
	GrantCommitment             [32]byte `json:"grant_commitment"`
	SessionIssuedAt             int64    `json:"session_issued_at"`
	SessionExpiresAt            int64    `json:"session_expires_at"`
	MemoryHighWater             uint64   `json:"memory_high_water"`
	CPUSeconds                  float64  `json:"cpu_seconds"`
	OpenFilesHighWater          uint32   `json:"open_files_high_water"`
	GoroutinesHighWater         uint32   `json:"goroutines_high_water"`
	ActiveSessions              uint32   `json:"active_sessions"`
	TimerHighWater              uint32   `json:"timer_high_water"`
	QueueHighWater              uint32   `json:"queue_high_water"`
	AcceptedIPCHighWater        uint32   `json:"accepted_ipc_high_water"`
	ServiceConnectionsHighWater uint32   `json:"service_connections_high_water"`
	ControlFilesHighWater       uint32   `json:"control_files_high_water"`
	ApplicationIPCAccepts       uint32   `json:"application_ipc_accepts"`
	RouteAttachmentsAccepted    uint32   `json:"route_attachments_accepted"`
}

// endpoint owns one broker generation's sessions and current publication.
type endpoint struct {
	mu                  sync.Mutex
	network, broker     [32]byte
	authority           [32]byte
	introduction        [32]byte
	connectionPrincipal [32]byte
	adminPrincipal      [32]byte
	sessions            map[[32]byte]localSession
	consumed            map[[32]byte]localSession
	current             *currentPublication
	lastGeneration      uint64
	generationStateFile string
	clock               func() time.Time
	resources           func(string, int) uint32
}

// New creates one finite Endpoint-local admission and publication boundary.
func New(input Setup) (*endpoint, error) {
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
	resources := input.Resources
	if resources == nil {
		resources = newResourceObserver()
	}
	return &endpoint{network: input.NetworkID, broker: input.BrokerID, authority: authority,
		introduction:        introduction,
		connectionPrincipal: input.ConnectionPrincipal, adminPrincipal: input.AdministrationPrincipal,
		sessions: make(map[[32]byte]localSession, maximumSessions), lastGeneration: lastGeneration,
		consumed: make(map[[32]byte]localSession, maximumSessions), generationStateFile: input.GenerationStateFile,
		clock: clock, resources: resources}, nil
}

// Do executes one admitted operation and returns only an R-002 product class.
func (endpoint *endpoint) Do(ctx context.Context, input Request) (Result, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	switch input.Action {
	case "admit", "publish", "unpublish", "connect", "accept":
	default:
		return denied("local operation is not permitted")
	}
	monitor := startResourceMonitor(endpoint.resources)
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
