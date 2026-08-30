package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

const (
	transitAcquisitionMarker  = "ardents-transit-acquisition-v1\n"
	transitAcquisitionSchema  = "ardents-transit-acquisition-state-v1"
	transitAcquisitionMaximum = int64(16 << 10)

	transitPending     transitAcquisitionPhase = "pending"
	transitReady       transitAcquisitionPhase = "ready"
	transitPresenting  transitAcquisitionPhase = "presenting"
	transitSpent       transitAcquisitionPhase = "spent"
	transitBurned      transitAcquisitionPhase = "burned"
	transitExhausted   transitAcquisitionPhase = "exhausted"
	transitWithdrawn   transitAcquisitionPhase = "withdrawn"
	transitUnavailable transitAcquisitionPhase = "unavailable"
)

var errTransitAcquisitionTerminal = errors.New("transit acquisition is terminal")

type transitAcquisitionPhase string

type transitAcquisitionOutcomeError struct {
	outcome credential.Outcome
}

func (failure transitAcquisitionOutcomeError) Error() string {
	return "transit acquisition is terminal: " + string(failure.outcome)
}

func (failure transitAcquisitionOutcomeError) Unwrap() error {
	return errTransitAcquisitionTerminal
}

type transitAcquisitionConfig struct {
	Root   string
	Create bool
	Clock  func() time.Time
}

type transitAcquisitionScope struct {
	NetworkID, Digest             [32]byte
	Epoch                         uint64
	IssuerNodeID, IssuerPublicKey [32]byte
	IssuerProfileDigest           [32]byte
	GrantSignerPublicKey          [32]byte
	IntroductionNodeID            [32]byte
	NotAfter                      time.Time
}

type transitAcquisitionAttempt struct {
	Phase       transitAcquisitionPhase
	Request     credential.Request
	Certificate tls.Certificate
	Grant       []byte
}

type transitAcquisitionState struct {
	Schema                         string                  `json:"schema"`
	Phase                          transitAcquisitionPhase `json:"phase"`
	NetworkID, Digest              [32]byte
	Epoch                          uint64
	IssuerNodeID, IssuerPublicKey  [32]byte
	IssuerProfileDigest            [32]byte
	GrantSignerPublicKey           [32]byte
	IntroductionNodeID             [32]byte
	NotAfter                       int64
	RequestID, AttachmentID        [32]byte
	ClientKeyDigest                [32]byte
	Certificate, PrivateKey, Grant []byte
}

type transitAcquisition struct {
	mu     sync.Mutex
	root   string
	clock  func() time.Time
	lease  transitAcquisitionLease
	state  transitAcquisitionState
	closed bool
	failed error
}

func openTransitAcquisition(config transitAcquisitionConfig) (*transitAcquisition, error) {
	if config.Root == "" || config.Clock == nil || config.Clock().IsZero() {
		return nil, errors.New("transit acquisition configuration is incomplete")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err := prepareTransitAcquisitionRoot(root, config.Create); err != nil {
		return nil, err
	}
	lease, err := acquireTransitAcquisitionLease(filepath.Join(root, "owner.lock"))
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := initializeTransitAcquisitionRoot(root, config.Create); err != nil {
		return nil, err
	}
	state, err := loadTransitAcquisitionState(root)
	if err != nil {
		return nil, err
	}
	owner := &transitAcquisition{root: root, clock: config.Clock, lease: lease, state: state}
	if state.Phase == transitPresenting {
		owner.state = terminalTransitAcquisition(state, transitBurned)
		if err := owner.commitState(owner.state); err != nil {
			return nil, err
		}
	} else if state.Phase != "" && !config.Clock().UTC().Before(time.Unix(state.NotAfter, 0).UTC()) && !terminalTransitPhase(state.Phase) {
		owner.state = terminalTransitAcquisition(state, transitUnavailable)
		if err := owner.commitState(owner.state); err != nil {
			return nil, err
		}
	}
	opened = true
	return owner, nil
}

func (owner *transitAcquisition) begin(scope transitAcquisitionScope) (transitAcquisitionAttempt, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if err := owner.usable(); err != nil {
		return transitAcquisitionAttempt{}, err
	}
	if !validTransitAcquisitionScope(scope, owner.clock().UTC()) {
		return transitAcquisitionAttempt{}, errors.New("transit acquisition scope is invalid")
	}
	if owner.state.Phase != "" {
		if terminalTransitPhase(owner.state.Phase) {
			if (owner.state.Phase == transitExhausted || owner.state.Phase == transitWithdrawn) && owner.state.matches(scope) {
				return transitAcquisitionAttempt{}, transitAcquisitionOutcomeError{outcome: credential.Outcome(owner.state.Phase)}
			}
			owner.state = transitAcquisitionState{}
		} else if !owner.state.matches(scope) {
			owner.state = terminalTransitAcquisition(owner.state, transitBurned)
			if err := owner.commitState(owner.state); err != nil {
				return transitAcquisitionAttempt{}, err
			}
			return transitAcquisitionAttempt{}, errors.New("transit acquisition State duty changed during an attempt")
		} else {
			return owner.attempt()
		}
	}
	certificate, err := route.NewClientCertificate()
	if err != nil {
		return transitAcquisitionAttempt{}, err
	}
	private, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(certificate.Certificate) != 1 || certificate.Leaf == nil {
		return transitAcquisitionAttempt{}, errors.New("transit acquisition TLS identity is invalid")
	}
	requestID, err := randomTransitAcquisitionID()
	if err != nil {
		return transitAcquisitionAttempt{}, err
	}
	attachmentID, err := randomTransitAcquisitionID()
	if err != nil {
		return transitAcquisitionAttempt{}, err
	}
	keyDigest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		return transitAcquisitionAttempt{}, err
	}
	owner.state = transitAcquisitionState{Schema: transitAcquisitionSchema, Phase: transitPending,
		NetworkID: scope.NetworkID, Digest: scope.Digest, Epoch: scope.Epoch, IssuerNodeID: scope.IssuerNodeID,
		IssuerPublicKey: scope.IssuerPublicKey, IssuerProfileDigest: scope.IssuerProfileDigest,
		GrantSignerPublicKey: scope.GrantSignerPublicKey, IntroductionNodeID: scope.IntroductionNodeID,
		NotAfter: scope.NotAfter.Unix(), RequestID: requestID, AttachmentID: attachmentID, ClientKeyDigest: keyDigest,
		Certificate: append([]byte(nil), certificate.Certificate[0]...), PrivateKey: append([]byte(nil), private...)}
	if err := owner.commitState(owner.state); err != nil {
		return transitAcquisitionAttempt{}, err
	}
	return owner.attempt()
}

func (owner *transitAcquisition) fail() error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if err := owner.usable(); err != nil {
		return err
	}
	if owner.state.Phase != transitPending && owner.state.Phase != transitReady {
		return errors.New("transit acquisition cannot fail from its current phase")
	}
	owner.state = terminalTransitAcquisition(owner.state, transitUnavailable)
	return owner.commitState(owner.state)
}

func (owner *transitAcquisition) commit(result credential.Result) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if err := owner.usable(); err != nil {
		return err
	}
	if owner.state.Phase != transitPending {
		return errors.New("transit acquisition is not pending")
	}
	switch result.Outcome {
	case credential.Issued:
		grant, err := route.VerifyTransitGrant(result.Grant, ed25519.PublicKey(owner.state.GrantSignerPublicKey[:]))
		if err != nil || !owner.state.matchesGrant(grant) {
			owner.state = terminalTransitAcquisition(owner.state, transitUnavailable)
			return errors.Join(errors.New("transit acquisition Grant is invalid"), owner.commitState(owner.state))
		}
		owner.state.Phase, owner.state.Grant = transitReady, append([]byte(nil), result.Grant...)
	case credential.Exhausted:
		owner.state = terminalTransitAcquisition(owner.state, transitExhausted)
	case credential.Withdrawn:
		owner.state = terminalTransitAcquisition(owner.state, transitWithdrawn)
	case credential.Unavailable:
		owner.state = terminalTransitAcquisition(owner.state, transitUnavailable)
	default:
		owner.state = terminalTransitAcquisition(owner.state, transitUnavailable)
		return errors.Join(errors.New("transit acquisition outcome is invalid"), owner.commitState(owner.state))
	}
	return owner.commitState(owner.state)
}

func (owner *transitAcquisition) present(scope transitAcquisitionScope) (transitAcquisitionAttempt, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if err := owner.usable(); err != nil {
		return transitAcquisitionAttempt{}, err
	}
	if owner.state.Phase != transitReady || !owner.state.matches(scope) {
		return transitAcquisitionAttempt{}, errors.New("transit acquisition is not ready for this State duty")
	}
	owner.state.Phase = transitPresenting
	if err := owner.commitState(owner.state); err != nil {
		return transitAcquisitionAttempt{}, err
	}
	return owner.attempt()
}

func (owner *transitAcquisition) finish(presented bool) error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if err := owner.usable(); err != nil {
		return err
	}
	if owner.state.Phase != transitPresenting {
		return errors.New("transit acquisition is not presenting")
	}
	phase := transitBurned
	if presented {
		phase = transitSpent
	}
	owner.state = terminalTransitAcquisition(owner.state, phase)
	return owner.commitState(owner.state)
}

func (owner *transitAcquisition) attempt() (transitAcquisitionAttempt, error) {
	certificate, err := owner.state.certificate()
	if err != nil {
		return transitAcquisitionAttempt{}, err
	}
	return transitAcquisitionAttempt{Phase: owner.state.Phase, Request: owner.state.request(), Certificate: certificate,
		Grant: append([]byte(nil), owner.state.Grant...)}, nil
}

func (owner *transitAcquisition) usable() error {
	if owner == nil || owner.closed {
		return errors.New("transit acquisition owner is closed")
	}
	if owner.failed != nil {
		return errors.New("transit acquisition owner requires restart after a failed commit")
	}
	return nil
}

func (owner *transitAcquisition) commitState(state transitAcquisitionState) error {
	if !validTransitAcquisitionState(state) {
		return errors.New("transit acquisition state is invalid")
	}
	raw, err := json.Marshal(state)
	if err == nil {
		err = replaceTransitAcquisitionState(owner.root, append(raw, '\n'))
	}
	if err != nil {
		owner.failed = err
	}
	return err
}

func (owner *transitAcquisition) stateForTest() transitAcquisitionState {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.state
}

func (owner *transitAcquisition) Close() error {
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return nil
	}
	owner.closed = true
	return owner.lease.release()
}

func (state transitAcquisitionState) request() credential.Request {
	return credential.Request{RequestID: state.RequestID, NetworkID: state.NetworkID, Digest: state.Digest, Epoch: state.Epoch,
		IntroductionNodeID: state.IntroductionNodeID, AttachmentID: state.AttachmentID,
		ClientKeyDigest: state.ClientKeyDigest, NotAfter: time.Unix(state.NotAfter, 0).UTC()}
}

func (state transitAcquisitionState) certificate() (tls.Certificate, error) {
	if len(state.Certificate) == 0 || len(state.PrivateKey) != ed25519.PrivateKeySize {
		return tls.Certificate{}, errors.New("transit acquisition TLS identity is unavailable")
	}
	leaf, err := x509.ParseCertificate(state.Certificate)
	if err != nil {
		return tls.Certificate{}, errors.New("transit acquisition TLS certificate is invalid")
	}
	private := ed25519.PrivateKey(append([]byte(nil), state.PrivateKey...))
	public, ok := private.Public().(ed25519.PublicKey)
	leafPublic, leafOK := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || !leafOK || !bytes.Equal(public, leafPublic) {
		return tls.Certificate{}, errors.New("transit acquisition TLS key does not match its certificate")
	}
	digest, err := route.ClientTLSKeyDigest(leaf)
	if err != nil || digest != state.ClientKeyDigest {
		return tls.Certificate{}, errors.New("transit acquisition TLS identity digest is invalid")
	}
	return tls.Certificate{Certificate: [][]byte{append([]byte(nil), state.Certificate...)}, PrivateKey: private, Leaf: leaf}, nil
}

func (state transitAcquisitionState) matches(scope transitAcquisitionScope) bool {
	return state.NetworkID == scope.NetworkID && state.Digest == scope.Digest && state.Epoch == scope.Epoch &&
		state.IssuerNodeID == scope.IssuerNodeID && state.IssuerPublicKey == scope.IssuerPublicKey &&
		state.IssuerProfileDigest == scope.IssuerProfileDigest && state.GrantSignerPublicKey == scope.GrantSignerPublicKey &&
		state.IntroductionNodeID == scope.IntroductionNodeID && state.NotAfter == scope.NotAfter.Unix()
}

func (state transitAcquisitionState) matchesGrant(grant route.TransitGrant) bool {
	return grant.NetworkID == state.NetworkID && grant.Digest == state.Digest && grant.Epoch == state.Epoch &&
		grant.TransitNodeID == state.IntroductionNodeID && grant.TransitRole == route.IntroductionRole &&
		grant.AttachmentID == state.AttachmentID && grant.ClientKeyDigest == state.ClientKeyDigest &&
		grant.NotAfter.Equal(time.Unix(state.NotAfter, 0).UTC())
}

func validTransitAcquisitionScope(scope transitAcquisitionScope, now time.Time) bool {
	return scope.NetworkID != [32]byte{} && scope.Digest != [32]byte{} && scope.Epoch != 0 &&
		scope.IssuerNodeID != [32]byte{} && scope.IssuerPublicKey != [32]byte{} && scope.IssuerProfileDigest != [32]byte{} &&
		scope.GrantSignerPublicKey != [32]byte{} && scope.IntroductionNodeID != [32]byte{} && now.Before(scope.NotAfter) &&
		scope.NotAfter.Equal(scope.NotAfter.UTC().Truncate(time.Second))
}

func validTransitAcquisitionState(state transitAcquisitionState) bool {
	if state.Phase == "" {
		return state.Schema == "" && state.NetworkID == [32]byte{} && state.Digest == [32]byte{} && state.Epoch == 0 &&
			state.IssuerNodeID == [32]byte{} && state.IssuerPublicKey == [32]byte{} && state.IssuerProfileDigest == [32]byte{} &&
			state.GrantSignerPublicKey == [32]byte{} && state.IntroductionNodeID == [32]byte{} && state.NotAfter == 0 &&
			state.RequestID == [32]byte{} && state.AttachmentID == [32]byte{} && state.ClientKeyDigest == [32]byte{} &&
			len(state.Certificate) == 0 && len(state.PrivateKey) == 0 && len(state.Grant) == 0
	}
	base := state.Schema == transitAcquisitionSchema && state.NetworkID != [32]byte{} && state.Digest != [32]byte{} && state.Epoch != 0 &&
		state.IssuerNodeID != [32]byte{} && state.IssuerPublicKey != [32]byte{} && state.IssuerProfileDigest != [32]byte{} &&
		state.GrantSignerPublicKey != [32]byte{} && state.IntroductionNodeID != [32]byte{} && state.NotAfter > 0 &&
		state.RequestID != [32]byte{} && state.AttachmentID != [32]byte{} && state.ClientKeyDigest != [32]byte{}
	if !base {
		return false
	}
	if terminalTransitPhase(state.Phase) {
		return len(state.Certificate) == 0 && len(state.PrivateKey) == 0 && len(state.Grant) == 0
	}
	if state.Phase != transitPending && state.Phase != transitReady && state.Phase != transitPresenting {
		return false
	}
	if len(state.Certificate) == 0 || len(state.PrivateKey) != ed25519.PrivateKeySize {
		return false
	}
	return state.Phase == transitPending && len(state.Grant) == 0 ||
		(state.Phase == transitReady || state.Phase == transitPresenting) && len(state.Grant) > 0 && len(state.Grant) <= 512
}

func terminalTransitPhase(phase transitAcquisitionPhase) bool {
	switch phase {
	case transitSpent, transitBurned, transitExhausted, transitWithdrawn, transitUnavailable:
		return true
	default:
		return false
	}
}

func terminalTransitAcquisition(state transitAcquisitionState, phase transitAcquisitionPhase) transitAcquisitionState {
	state.Phase, state.Certificate, state.PrivateKey, state.Grant = phase, nil, nil, nil
	return state
}

func randomTransitAcquisitionID() ([32]byte, error) {
	var result [32]byte
	_, err := rand.Read(result[:])
	return result, err
}
