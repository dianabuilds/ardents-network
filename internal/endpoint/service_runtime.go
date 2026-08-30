package endpoint

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"sync"
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
	browserCompatibilitySetup
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

// PublicationRequest contains only the Administrator-authorized facts needed
// to publish one current Instance generation.
type PublicationRequest struct {
	Principal, Capability       [32]byte
	Credential                  Credential
	InstanceSigner              crypto.Signer
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
	browserCompatibility
	network, broker [32]byte
	authority       [32]byte
	introduction    [32]byte
	admission       *broker.Broker
	publications    *publication.Publication
	resources       func(string, int) uint32
	transitClients  map[[32]byte]tls.Certificate
	transitAcquire  *transitAcquisition
	transitMu       sync.Mutex
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
	transitClients, err := cloneTransitClientCertificates(input.TransitClientCertificates)
	if err != nil {
		return nil, err
	}
	var transitAcquire *transitAcquisition
	if input.TransitAcquisitionRoot != "" {
		transitAcquire, err = openTransitAcquisition(transitAcquisitionConfig{Root: input.TransitAcquisitionRoot,
			Create: input.CreateTransitAcquisitionRoot, Clock: clock})
		if err != nil {
			return nil, err
		}
	}
	endpoint := &endpoint{network: input.NetworkID, broker: input.BrokerID, authority: authority,
		introduction: introduction,
		admission:    admission, resources: resources, transitClients: transitClients, transitAcquire: transitAcquire}
	if input.AdministrationPrincipal != [32]byte{} {
		if input.PublicationRoot == "" {
			if transitAcquire != nil {
				_ = transitAcquire.Close()
			}
			return nil, errors.New("publisher setup lacks a publication root")
		}
		opened, err := publication.Open(publication.Config{Root: input.PublicationRoot,
			LegacyFloor: input.LegacyGenerationFloor, NetworkID: input.NetworkID,
			Authority: input.AuthorityPublic, Clock: clock})
		if err != nil {
			if transitAcquire != nil {
				_ = transitAcquire.Close()
			}
			return nil, err
		}
		endpoint.publications = opened
	}
	compatibility, err := openBrowserCompatibility(input.browserCompatibilitySetup)
	if err != nil {
		if endpoint.publications != nil {
			_ = endpoint.publications.Close()
		}
		if transitAcquire != nil {
			_ = transitAcquire.Close()
		}
		return nil, err
	}
	endpoint.browserCompatibility = compatibility
	return endpoint, nil
}

// Close withdraws the publisher's live Instance and releases its publication
// root. Client-only endpoints have no publication owner to close.
func (endpoint *endpoint) Close() error {
	if endpoint == nil {
		return nil
	}
	endpoint.admission.Close()
	compatibilityErr := endpoint.closeBrowserCompatibility()
	acquisitionErr := endpoint.transitAcquire.Close()
	if endpoint.publications == nil {
		return errors.Join(compatibilityErr, acquisitionErr)
	}
	return errors.Join(compatibilityErr, acquisitionErr, endpoint.publications.Close())
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
