package endpoint

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const (
	publishCapability  = uint32(1)
	connectCapability  = uint32(2)
	maximumStreamBytes = uint32(768 << 20)
)

// Recovery is owned by the native connection lifecycle.
type Recovery = nativeconnection.Recovery

// DestinationBinding is owned by the native connection lifecycle.
type DestinationBinding = nativeconnection.DestinationBinding

// Setup fixes one Endpoint broker generation and its two local principals.
type Setup struct {
	NetworkID               [32]byte
	BrokerID                [32]byte
	AuthorityPublic         ed25519.PublicKey
	IntroductionPublic      ed25519.PublicKey
	ConnectionPrincipal     [32]byte
	AdministrationPrincipal [32]byte
	PublicationRoot         string
	LegacyGenerationFloor   string
	Clock                   func() time.Time
	Resources               func(string, int) uint32
	Admission               *broker.Broker
}

// Request is the remaining native Connection or withdrawal operation input.
// Publication material is intentionally carried only by PublicationRequest.
type Request struct {
	Action                     string
	Principal, Session, Target [32]byte
	Publication                []byte
	Route                      net.Conn
	Application                io.ReadWriteCloser
	OpenAttachment             func(context.Context, Recovery) (net.Conn, error)
	RecoveryBinding            Recovery
	NameBinding                DestinationBinding
	NameUpdates                <-chan DestinationBinding
	BytesEachDirection         uint32
	SendBytes, ReceiveBytes    uint32
	At                         time.Time
}

// PublicationRequest contains only the Administrator-authorized facts needed
// to publish one current Instance generation.
type PublicationRequest struct {
	Principal, Capability       [32]byte
	Credential                  Credential
	InstancePrivate             ed25519.PrivateKey
	IntroductionAcknowledgement []byte
	IntroductionSocket          string
	At                          time.Time
}

// PublicationResult is the bounded public record and its exact admission
// receipt. It contains no Route, Application, or connection facts.
type PublicationResult struct {
	Class                       string
	Reason                      string
	Record                      []byte
	AuthenticatedTarget         [32]byte
	Generation                  uint64
	IntroductionReceipt         [32]byte
	IntroductionAcknowledgement []byte
	Receipt                     broker.Receipt
}

// WithdrawalRequest contains only the Administration capability needed to
// release the current publication.
type WithdrawalRequest struct {
	Principal, Capability [32]byte
	At                    time.Time
}

// WithdrawalResult reports the released public generation and its exact
// admission receipt. It contains no connection facts.
type WithdrawalResult struct {
	Class               string
	Reason              string
	AuthenticatedTarget [32]byte
	Generation          uint64
	Receipt             broker.Receipt
}

// RuntimeResult is a bounded endpoint-runtime outcome with no Route internals.
// It is transitional while Endpoint replaces the former action/result union
// with role-specific internal operations.
type RuntimeResult struct {
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
	network, broker [32]byte
	authority       [32]byte
	introduction    [32]byte
	admission       *broker.Broker
	publications    *publication.Publication
	resources       func(string, int) uint32
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
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	resources := input.Resources
	if resources == nil {
		resources = newResourceObserver()
	}
	admission := input.Admission
	if admission == nil {
		grants := []broker.Grant{{Principal: input.ConnectionPrincipal, Surface: broker.Connection}}
		if input.AdministrationPrincipal != [32]byte{} {
			grants = append(grants, broker.Grant{Principal: input.AdministrationPrincipal, Surface: broker.Administration})
		}
		openedAdmission, err := broker.New(broker.Config{ID: input.BrokerID, Grants: grants, Clock: clock})
		if err != nil {
			return nil, err
		}
		admission = openedAdmission
	}
	endpoint := &endpoint{network: input.NetworkID, broker: input.BrokerID, authority: authority,
		introduction: introduction,
		admission:    admission, resources: resources}
	if input.AdministrationPrincipal != [32]byte{} {
		if input.PublicationRoot == "" {
			return nil, errors.New("publisher setup lacks a publication root")
		}
		opened, err := publication.Open(publication.Config{Root: input.PublicationRoot,
			LegacyFloor: input.LegacyGenerationFloor, NetworkID: input.NetworkID,
			Authority: input.AuthorityPublic, Clock: clock})
		if err != nil {
			return nil, err
		}
		endpoint.publications = opened
	}
	return endpoint, nil
}

// Close withdraws the publisher's live Instance and releases its publication
// root. Client-only endpoints have no publication owner to close.
func (endpoint *endpoint) Close() error {
	if endpoint == nil {
		return nil
	}
	endpoint.admission.Close()
	if endpoint.publications == nil {
		return nil
	}
	return endpoint.publications.Close()
}

// Publish consumes one Administration capability before publishing an exact
// current Instance generation. It never accepts Route or Application facts.
func (endpoint *endpoint) Publish(ctx context.Context, input PublicationRequest) (PublicationResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return publicationDenied("local publication is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return publicationFailed("local timeout or cancellation", "local publication was cancelled", err)
	}
	return endpoint.publish(ctx, input)
}

// Withdraw consumes one Administration capability before withdrawing the
// current Instance publication.
func (endpoint *endpoint) Withdraw(ctx context.Context, input WithdrawalRequest) (WithdrawalResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return withdrawalDenied("local withdrawal is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return withdrawalFailed("local timeout or cancellation", "local withdrawal was cancelled", err)
	}
	return endpoint.unpublish(ctx, input)
}

// Do executes one admitted operation and returns only an R-002 product class.
func (endpoint *endpoint) Do(ctx context.Context, input Request) (RuntimeResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	switch input.Action {
	case "connect", "accept":
	default:
		return denied("local operation is not permitted")
	}
	monitor := startResourceMonitor(endpoint.resources)
	var result RuntimeResult
	var err error
	switch input.Action {
	case "connect":
		result, err = endpoint.connect(ctx, input)
	case "accept":
		result, err = endpoint.accept(ctx, input)
	}
	endpoint.observe(&result, input, monitor.stop())
	return result, err
}

func publicationDenied(reason string) (PublicationResult, error) {
	return publicationFailed("local authorization or policy denial", reason, errors.New(reason))
}

func publicationFailed(class, reason string, err error) (PublicationResult, error) {
	return PublicationResult{Class: class, Reason: reason}, err
}

func withdrawalDenied(reason string) (WithdrawalResult, error) {
	return withdrawalFailed("local authorization or policy denial", reason, errors.New(reason))
}

func withdrawalFailed(class, reason string, err error) (WithdrawalResult, error) {
	return WithdrawalResult{Class: class, Reason: reason}, err
}

func denied(reason string) (RuntimeResult, error) {
	return failed("local authorization or policy denial", reason, errors.New(reason))
}

func failed(class, reason string, err error) (RuntimeResult, error) {
	return RuntimeResult{Class: class, Reason: reason}, err
}
