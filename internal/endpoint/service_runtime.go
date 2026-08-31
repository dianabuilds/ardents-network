package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
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
	// PublisherBinding and PublisherIntroductionProfile are the Endpoint-owned
	// accepted Instance and State-selected slot facts for the maintained
	// headless Publisher start path. They are never supplied by an
	// Administration caller.
	PublisherBinding             *instance.Binding
	PublisherIntroductionProfile PublisherIntroductionProfile
	Clock                        func() time.Time
	Resources                    func(string, int) uint32
	Admission                    *broker.Broker
	// TransitClientCertificates holds the private, one-use TLS identities
	// provisioned with State-authorized Transit Grants. Its keys are opaque
	// Grant IDs; neither the map nor its private keys are Service Descriptor
	// material or browser-facing configuration.
	TransitClientCertificates map[[32]byte]tls.Certificate
	// TransitAcquisitionRoot is the exclusive owner-only Endpoint journal for
	// one in-flight membership Transit Grant acquisition. Create is explicit so
	// an uninitialized or substituted participant root cannot be claimed by a
	// normal reopen.
	TransitAcquisitionRoot       string
	CreateTransitAcquisitionRoot bool
}

// connectionInput is the private common carrier input derived only from one
// typed inbound or outbound request.
type connectionInput struct {
	Principal, Session, Target       [32]byte
	AuthorityPublic                  [32]byte
	Publication                      []byte
	Route                            net.Conn
	Application                      io.ReadWriteCloser
	OpenAttachment                   func(context.Context, Recovery) (net.Conn, error)
	OnAuthenticated                  func([32]byte) error
	RecoveryBinding                  Recovery
	NameBinding                      DestinationBinding
	NameUpdates                      <-chan DestinationBinding
	closeApplicationOnRemoteTerminal bool
	BytesEachDirection               uint32
	SendBytes, ReceiveBytes          uint32
	At                               time.Time
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

// PublisherStartRequest is the complete local Application input for starting
// the Endpoint-owned Publisher generation. All network and Service authority
// facts remain inside Endpoint composition.
type PublisherStartRequest struct {
	Principal, Capability [32]byte
	At                    time.Time
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

// OutboundConnectionRequest contains exactly the client-side facts for one
// authenticated Connection. It has no publisher publication owner or signer.
type OutboundConnectionRequest struct {
	Principal, Capability, Target    [32]byte
	AuthorityPublic                  [32]byte
	Publication                      []byte
	Route                            net.Conn
	Application                      io.ReadWriteCloser
	OpenAttachment                   func(context.Context, Recovery) (net.Conn, error)
	OnAuthenticated                  func([32]byte) error
	RecoveryBinding                  Recovery
	NameBinding                      DestinationBinding
	NameUpdates                      <-chan DestinationBinding
	closeApplicationOnRemoteTerminal bool
	BytesEachDirection               uint32
	SendBytes, ReceiveBytes          uint32
	At                               time.Time
}

// InboundConnectionRequest contains exactly the publisher-side facts for one
// authenticated Connection. It has no public record or resolved target input.
type InboundConnectionRequest struct {
	Principal, Capability   [32]byte
	Route                   net.Conn
	Application             io.ReadWriteCloser
	OpenAttachment          func(context.Context, Recovery) (net.Conn, error)
	RecoveryBinding         Recovery
	BytesEachDirection      uint32
	SendBytes, ReceiveBytes uint32
	At                      time.Time
}

// RuntimeResult is a bounded endpoint-runtime outcome with no Route internals.
// Its remaining evidence projection is reduced independently of the
// role-specific operation inputs.
type RuntimeResult struct {
	Class                    string         `json:"class"`
	Reason                   string         `json:"reason"`
	AuthenticatedTarget      [32]byte       `json:"authenticated_target"`
	Generation               uint64         `json:"generation"`
	RouteGeneration          uint64         `json:"route_generation"`
	RecoveryCount            uint32         `json:"recovery_count"`
	ContinuityCommitment     [32]byte       `json:"continuity_commitment"`
	AcceptedBytes            uint32         `json:"accepted_bytes"`
	AcknowledgedBytes        uint32         `json:"acknowledged_bytes"`
	ReceivedBytes            uint32         `json:"received_bytes"`
	Admission                broker.Receipt `json:"admission"`
	QueueHighWater           uint32         `json:"queue_high_water"`
	ApplicationIPCAccepts    uint32         `json:"application_ipc_accepts"`
	RouteAttachmentsAccepted uint32         `json:"route_attachments_accepted"`
}

// endpoint owns one broker generation's sessions and current publication.
type endpoint struct {
	network, broker      [32]byte
	authority            [32]byte
	introduction         [32]byte
	admission            *broker.Broker
	publications         *publication.Publication
	resources            func(string, int) uint32
	transitClients       map[[32]byte]tls.Certificate
	transitAcquire       *transitAcquisitionSet
	transitMu            sync.Mutex
	publisherMu          sync.Mutex
	publisherBinding     *instance.Binding
	publisherProfile     PublisherIntroductionProfile
	publisherSession     *PublisherIntroduction
	publisherPrepare     func(context.Context, time.Time) (acquiredPublisherProfile, error)
	publisherCredentials publisherCredentialCompletions
}

// New creates one finite Endpoint-local admission and publication boundary.
func New(input Setup) (*endpoint, error) {
	if input.NetworkID == [32]byte{} || input.BrokerID == [32]byte{} || input.ConnectionPrincipal == [32]byte{} ||
		len(input.AuthorityPublic) != 0 && len(input.AuthorityPublic) != ed25519.PublicKeySize ||
		len(input.IntroductionPublic) != 0 && len(input.IntroductionPublic) != ed25519.PublicKeySize {
		return nil, errors.New("endpoint setup is incomplete")
	}
	var authority [32]byte
	copy(authority[:], input.AuthorityPublic)
	var introduction [32]byte
	copy(introduction[:], input.IntroductionPublic)
	if input.PublisherBinding != nil {
		credential := input.PublisherBinding.Credential()
		if credential.NetworkID != input.NetworkID ||
			authority != [32]byte{} && authority != credential.AuthorityPublic ||
			introduction != [32]byte{} && introduction != credential.IntroductionHPKEPublic {
			return nil, errors.New("publisher binding does not match Endpoint setup")
		}
		authority, introduction = credential.AuthorityPublic, credential.IntroductionHPKEPublic
	}
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
	transitClients, err := cloneTransitClientCertificates(input.TransitClientCertificates)
	if err != nil {
		return nil, err
	}
	var transitAcquire *transitAcquisitionSet
	if input.TransitAcquisitionRoot != "" {
		transitAcquire, err = openTransitAcquisitionSet(transitAcquisitionConfig{Root: input.TransitAcquisitionRoot,
			Create: input.CreateTransitAcquisitionRoot, Clock: clock})
		if err != nil {
			return nil, err
		}
	}
	endpoint := &endpoint{network: input.NetworkID, broker: input.BrokerID, authority: authority,
		introduction: introduction,
		admission:    admission, resources: resources, transitClients: transitClients, transitAcquire: transitAcquire,
		publisherBinding: input.PublisherBinding, publisherProfile: clonePublisherIntroductionProfile(input.PublisherIntroductionProfile)}
	if input.PublisherBinding != nil && (input.AdministrationPrincipal == [32]byte{} ||
		input.PublisherIntroductionProfile.NetworkID != input.NetworkID || !validPublisherIntroductionProfile(input.PublisherIntroductionProfile)) {
		if transitAcquire != nil {
			_ = transitAcquire.Close()
		}
		return nil, errors.New("publisher start ownership is incomplete")
	}
	if input.AdministrationPrincipal != [32]byte{} && (input.PublisherBinding != nil || authority != [32]byte{}) {
		if input.PublicationRoot == "" {
			if transitAcquire != nil {
				_ = transitAcquire.Close()
			}
			return nil, errors.New("publisher setup lacks a publication root")
		}
		opened, err := publication.Open(publication.Config{Root: input.PublicationRoot,
			LegacyFloor: input.LegacyGenerationFloor, NetworkID: input.NetworkID,
			Authority: ed25519.PublicKey(authority[:]), Clock: clock})
		if err != nil {
			if transitAcquire != nil {
				_ = transitAcquire.Close()
			}
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
	endpoint.publisherMu.Lock()
	session, binding := endpoint.publisherSession, endpoint.publisherBinding
	credentials := endpoint.publisherCredentials
	endpoint.publisherSession = nil
	endpoint.publisherBinding = nil
	endpoint.publisherPrepare = nil
	endpoint.publisherCredentials = publisherCredentialCompletions{}
	endpoint.publisherMu.Unlock()
	var sessionErr error
	if session != nil {
		sessionErr = session.Close()
	}
	finishErr := errors.Join(finishTransitCredential(credentials.introduction, false),
		finishTransitCredential(credentials.responder, false))
	acquisitionErr := endpoint.transitAcquire.Close()
	if endpoint.publications == nil {
		return errors.Join(sessionErr, finishErr, acquisitionErr)
	}
	publicationErr := endpoint.publications.Close()
	var bindingErr error
	if binding != nil {
		bindingErr = binding.Withdraw()
	}
	return errors.Join(sessionErr, finishErr, acquisitionErr, publicationErr, bindingErr)
}

// StartPublisher consumes one Administration capability and atomically binds
// the Endpoint-owned Instance generation to its live Introduction slot.
func (endpoint *endpoint) StartPublisher(ctx context.Context, input PublisherStartRequest) (PublicationResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return publicationDenied("local Publisher start is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return publicationFailed("local timeout or cancellation", "local Publisher start was cancelled", err)
	}
	return endpoint.startPublisher(ctx, input)
}

// AcceptPublisher hands one local Publisher Application stream to the
// Endpoint-owned live Introduction session. The caller cannot supply a Route
// or recovery attachment.
func (endpoint *endpoint) AcceptPublisher(ctx context.Context, input InboundConnectionRequest) (RuntimeResult, error) {
	if endpoint == nil || ctx == nil {
		return denied("local Publisher acceptance is incomplete")
	}
	endpoint.publisherMu.Lock()
	session := endpoint.publisherSession
	endpoint.publisherMu.Unlock()
	if session == nil {
		return failed("service unavailable", "Publisher Introduction session is unavailable", errors.New("publisher is not started"))
	}
	return session.Accept(ctx, input)
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

// Connect runs one client-side native Connection with only outbound facts.
func (endpoint *endpoint) Connect(ctx context.Context, input OutboundConnectionRequest) (RuntimeResult, error) {
	return endpoint.runOutbound(ctx, connectionInput{Principal: input.Principal, Session: input.Capability,
		Target: input.Target, AuthorityPublic: input.AuthorityPublic, Publication: input.Publication, Route: input.Route, Application: input.Application,
		OpenAttachment: input.OpenAttachment, RecoveryBinding: input.RecoveryBinding, NameBinding: input.NameBinding,
		NameUpdates: input.NameUpdates, closeApplicationOnRemoteTerminal: input.closeApplicationOnRemoteTerminal, OnAuthenticated: input.OnAuthenticated, BytesEachDirection: input.BytesEachDirection, SendBytes: input.SendBytes,
		ReceiveBytes: input.ReceiveBytes, At: input.At})
}

// Accept runs one publisher-side native Connection with only inbound facts.
func (endpoint *endpoint) Accept(ctx context.Context, input InboundConnectionRequest) (RuntimeResult, error) {
	return endpoint.runInbound(ctx, connectionInput{Principal: input.Principal, Session: input.Capability,
		Route: input.Route, Application: input.Application, OpenAttachment: input.OpenAttachment,
		RecoveryBinding: input.RecoveryBinding, BytesEachDirection: input.BytesEachDirection,
		SendBytes: input.SendBytes, ReceiveBytes: input.ReceiveBytes, At: input.At})
}

func (endpoint *endpoint) runOutbound(ctx context.Context, input connectionInput) (RuntimeResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	return endpoint.connect(ctx, input)
}

func (endpoint *endpoint) runInbound(ctx context.Context, input connectionInput) (RuntimeResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	return endpoint.accept(ctx, input)
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
