package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

type testAccessGrantIssuer struct {
	key    ed25519.PrivateKey
	mutate func(*identityprotocol.AccessGrantPayload)
	err    error
}

func (i testAccessGrantIssuer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), i.key.Public().(ed25519.PublicKey)...)
}

func (i testAccessGrantIssuer) IssueAccessGrant(payload *identityprotocol.AccessGrantPayload) (*Artifact, error) {
	if i.err != nil {
		return nil, i.err
	}
	if i.mutate != nil {
		i.mutate(payload)
	}
	return SignAccessGrant(payload, i.key)
}

func (i testAccessGrantIssuer) IssueAccessGrantRevocation(payload *identityprotocol.AccessGrantRevocationPayload, grant *Artifact) (*Artifact, error) {
	return SignAccessGrantRevocation(payload, i.key, payload.RevokedAt.AsTime(), grant)
}

func (i testAccessGrantIssuer) IssueDeviceRevocation(payload *identityprotocol.DeviceRevocationPayload) (*Artifact, error) {
	return SignDeviceRevocation(payload, i.key, payload.RevokedAt.AsTime())
}

type rollbackAfterUpdateDatabase struct {
	storage.Database
	fail atomic.Bool
}

func (d *rollbackAfterUpdateDatabase) Update(ctx context.Context, callback func(storage.WriteTransaction) error) error {
	return d.Database.Update(ctx, func(tx storage.WriteTransaction) error {
		if err := callback(tx); err != nil {
			return err
		}
		if d.fail.Load() {
			return errors.New("injected transaction rollback")
		}
		return nil
	})
}

func (f *serviceFixture) firstEnrollmentRequest(ticket BootstrapTicket) FirstEnrollmentRequest {
	f.t.Helper()
	challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF)
	signed, err := challengeSigningBytes(challenge)
	require.NoError(f.t, err)
	var root [32]byte
	copy(root[:], f.root.Public().(ed25519.PublicKey))
	result, err := f.service.Complete(f.ctx, CompleteRequest{ChallengeID: challenge.ID, Principal: f.principal, Binding: f.binding, Source: f.source, RootPublicKey: root, Signature: ed25519.Sign(f.root, signed)})
	require.NoError(f.t, err)
	require.NotNil(f.t, result.EnrollmentProof)
	credential, err := f.credential.MarshalBinary()
	require.NoError(f.t, err)
	return FirstEnrollmentRequest{Ticket: ticket, Challenge: challenge, Proof: *result.EnrollmentProof, RootPublicKey: root, Credential: credential}
}

func TestFirstEnrollmentAtomicallyPersistsPrincipalAndNodeSignedRecoveryGrant(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	request := f.firstEnrollmentRequest(ticket)

	result, err := f.service.EnrollFirstPrincipal(f.ctx, f.binding, request)
	require.NoError(t, err)
	require.Equal(t, f.principal, result.Principal)
	require.Equal(t, f.credential.ID(), result.CredentialID)
	require.NotEmpty(t, result.GrantID)
	loaded, err := f.service.enrollments.load(f.ctx, f.nodeID, f.principal)
	require.NoError(t, err)
	require.Equal(t, f.principal, loaded.Principal)
	var grant *Artifact
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		var loadErr error
		grant, loadErr = loadGrant(tx, result.GrantID, f.clock.Now())
		return loadErr
	}))
	payload := grant.AccessGrantPayload()
	require.Equal(t, initialOperatorRecoveryActions, payload.Actions)
	require.Equal(t, f.nodeID, payload.Issuer)
	require.Equal(t, f.principal, payload.Subject)
	require.Equal(t, identityprotocol.Interface_INTERFACE_OPERATOR, payload.Audience.Interface)
	require.IsType(t, &identityprotocol.ResourceScope_Node{}, payload.Scope.Scope)
	require.ErrorIs(t, consumeBootstrapForTest(f.ctx, f.database, f.nodeID, ticket, f.clock.Now()), ErrConflict)
}

func TestFirstEnrollmentRejectsWrongPrincipalWithoutConsumingTicket(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	request := f.firstEnrollmentRequest(ticket)
	wrongRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x91}, 32))
	copy(request.RootPublicKey[:], wrongRoot.Public().(ed25519.PublicKey))
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, request)
	require.ErrorIs(t, err, ErrInvalidArgument)

	valid := f.firstEnrollmentRequest(ticket)
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, valid)
	require.NoError(t, err)
}

func TestFirstEnrollmentRejectsCurrentTransportSubstitutionBeforeConsumingProof(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	request := f.firstEnrollmentRequest(ticket)
	wrongPeer := f.binding
	wrongPeer.PeerBinding[0] ^= 0xff
	_, err = f.service.EnrollFirstPrincipal(f.ctx, wrongPeer, request)
	require.ErrorIs(t, err, ErrInvalidArgument)
	wrongInterface := f.binding
	wrongInterface.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	_, err = f.service.EnrollFirstPrincipal(f.ctx, wrongInterface, request)
	require.ErrorIs(t, err, ErrInvalidArgument)
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, request)
	require.NoError(t, err)
}

func TestFirstEnrollmentRejectsWrongOrMutatingNodeIssuerWithoutConsumingTicket(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	wrongNode := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x92}, 32))
	f.service.grantIssuer = testAccessGrantIssuer{key: wrongNode}
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, f.firstEnrollmentRequest(ticket))
	require.ErrorIs(t, err, ErrUnavailable)

	f.service.grantIssuer = testAccessGrantIssuer{key: f.node, mutate: func(payload *identityprotocol.AccessGrantPayload) {
		payload.Actions = []string{"node.status"}
	}}
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, f.firstEnrollmentRequest(ticket))
	require.ErrorIs(t, err, ErrUnavailable)

	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, f.firstEnrollmentRequest(ticket))
	require.NoError(t, err)
}

func TestConcurrentFirstEnrollmentHasOneCommittedWinner(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	requests := []FirstEnrollmentRequest{f.firstEnrollmentRequest(ticket), f.firstEnrollmentRequest(ticket)}
	var successes atomic.Int32
	var failures atomic.Int32
	var wg sync.WaitGroup
	for _, request := range requests {
		request := request
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, enrollErr := f.service.EnrollFirstPrincipal(context.Background(), f.binding, request)
			if enrollErr == nil {
				successes.Add(1)
			} else if errors.Is(enrollErr, ErrUnauthenticated) || errors.Is(enrollErr, ErrConflict) {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	require.Equal(t, int32(1), failures.Load())
}

func TestFirstEnrollmentTransactionFailureRollsBackAllDurableWrites(t *testing.T) {
	f := newServiceFixture(t)
	wrapped := &rollbackAfterUpdateDatabase{Database: f.database}
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	f.service.grants.database = wrapped
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	request := f.firstEnrollmentRequest(ticket)
	wrapped.fail.Store(true)
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, request)
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = f.service.enrollments.load(f.ctx, f.nodeID, f.principal)
	require.Error(t, err)
	wrapped.fail.Store(false)
	request = f.firstEnrollmentRequest(ticket)
	_, err = f.service.EnrollFirstPrincipal(f.ctx, f.binding, request)
	require.NoError(t, err)
}

func TestFirstEnrollmentRequestRedactsTicketProofCredentialAndChallenge(t *testing.T) {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	request := f.firstEnrollmentRequest(ticket)
	formatted := fmt.Sprintf("%v %#v", request, request)
	require.NotContains(t, formatted, string(request.Ticket[:]))
	require.NotContains(t, formatted, string(request.Proof[:]))
	require.NotContains(t, formatted, string(request.Credential))
	raw, err := json.Marshal(request)
	require.NoError(t, err)
	require.Equal(t, `{"protected":"[redacted]"}`, string(raw))
}
