package access

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type applicationEnrollmentFixture struct {
	*adminFixture
	appBinding   AuthenticationBinding
	appSource    SourceKey
	appRoot      ed25519.PrivateKey
	appDevice    ed25519.PrivateKey
	appPrincipal string
	credential   []byte
}

func newApplicationEnrollmentFixture(t *testing.T) *applicationEnrollmentFixture {
	t.Helper()
	f := newAdminFixture(t)
	f.service.applicationEnrollmentEnabled = true
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credential, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(f.clock.Now().Add(-time.Minute)), NotAfter: timestamppb.New(f.clock.Now().Add(time.Hour)),
	}, root)
	require.NoError(t, err)
	credentialRaw, err := credential.MarshalBinary()
	require.NoError(t, err)
	appBinding := f.binding
	appBinding.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	appBinding.PeerBinding[0] ^= 0x7f
	appSource := f.source
	appSource[0] ^= 0x5f
	return &applicationEnrollmentFixture{adminFixture: f, appBinding: appBinding, appSource: appSource, appRoot: root, appDevice: device, appPrincipal: principal.String(), credential: credentialRaw}
}

func (f *applicationEnrollmentFixture) issueTicket() ApplicationEnrollmentTicketResult {
	f.t.Helper()
	result, err := f.service.IssueApplicationEnrollmentTicket(f.ctx, IssueApplicationEnrollmentTicketRequest{
		Attempt:   f.command("application-ticket", "identity.principal.enroll", "principal", f.appPrincipal).Attempt,
		Principal: f.appPrincipal, Actions: []Action{"application.content.get", "application.content.put"},
	})
	require.NoError(f.t, err)
	return result
}

func (f *applicationEnrollmentFixture) enrollmentRequest(ticket ApplicationEnrollmentTicket) EnrollApplicationRequest {
	f.t.Helper()
	challenge, err := f.service.Begin(f.ctx, BeginRequest{Principal: f.appPrincipal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, Binding: f.appBinding, Source: f.appSource})
	require.NoError(f.t, err)
	signed, err := challengeSigningBytes(challenge)
	require.NoError(f.t, err)
	var root [ed25519.PublicKeySize]byte
	copy(root[:], f.appRoot.Public().(ed25519.PublicKey))
	proof, err := f.service.Complete(f.ctx, CompleteRequest{ChallengeID: challenge.ID, Principal: f.appPrincipal, Binding: f.appBinding, Source: f.appSource, RootPublicKey: root, Signature: ed25519.Sign(f.appRoot, signed)})
	require.NoError(f.t, err)
	require.NotNil(f.t, proof.EnrollmentProof)
	return EnrollApplicationRequest{Ticket: ticket, Challenge: challenge, Proof: *proof.EnrollmentProof, RootPublicKey: root, Credential: append([]byte(nil), f.credential...)}
}

func TestApplicationEnrollmentIsFeatureGatedAndRejectsForgedTicket(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	f.service.applicationEnrollmentEnabled = false
	_, err := f.service.IssueApplicationEnrollmentTicket(f.ctx, IssueApplicationEnrollmentTicketRequest{
		Attempt: f.command("disabled", "identity.principal.enroll", "principal", f.appPrincipal).Attempt, Principal: f.appPrincipal,
		Actions: []Action{"application.content.get"},
	})
	require.ErrorIs(t, err, ErrFeatureDisabled)

	f.service.applicationEnrollmentEnabled = true
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	copy(request.Ticket[:], bytes.Repeat([]byte{0x7f}, len(request.Ticket)))
	_, err = f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestApplicationEnrollmentTicketIssueRejectsMalformedPolicyAndMissingBinding(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	base := IssueApplicationEnrollmentTicketRequest{
		Attempt:   f.command("application-ticket", "identity.principal.enroll", "principal", f.appPrincipal).Attempt,
		Principal: f.appPrincipal,
	}
	for name, actions := range map[string][]Action{
		"unknown":      {"application.content.delete"},
		"operator":     {"node.status"},
		"duplicate":    {"application.content.get", "application.content.get"},
		"out of order": {"application.content.put", "application.content.get"},
	} {
		t.Run(name, func(t *testing.T) {
			request := base
			request.Actions = actions
			_, err := f.service.IssueApplicationEnrollmentTicket(f.ctx, request)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})
	}

}

func TestApplicationEnrollmentTicketIssueRejectsZeroEntropy(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	f.service.entropy = bytes.NewReader(make([]byte, identitycontract.ApplicationEnrollmentTicketBytes))
	_, err := f.service.IssueApplicationEnrollmentTicket(f.ctx, IssueApplicationEnrollmentTicketRequest{
		Attempt:   f.command("application-ticket", "identity.principal.enroll", "principal", f.appPrincipal).Attempt,
		Principal: f.appPrincipal, Actions: []Action{"application.content.get"},
	})
	require.ErrorIs(t, err, ErrInternal)
}

func TestApplicationEnrollmentAtomicallyPersistsCredentialAndGrant(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	result, err := f.service.EnrollApplication(f.ctx, f.appBinding, f.enrollmentRequest(ticket.Ticket))
	require.NoError(t, err)
	require.Equal(t, f.appPrincipal, result.Principal)
	require.NotEmpty(t, result.CredentialID)
	require.NotEmpty(t, result.GrantID)
	require.Equal(t, f.clock.Now().Add(identitycontract.DefaultGrantLifetime), result.GrantExpiresAt)

	enrolled, err := f.service.enrollments.load(f.ctx, f.nodeID, f.appPrincipal)
	require.NoError(t, err)
	require.Equal(t, f.appPrincipal, enrolled.Principal)

	var grant *Artifact
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		var loadErr error
		grant, loadErr = loadGrant(tx, result.GrantID, f.clock.Now())
		return loadErr
	}))
	require.Equal(t, []string{"application.content.get", "application.content.put"}, grant.AccessGrantPayload().Actions)
	require.Equal(t, identityprotocol.Interface_INTERFACE_APPLICATION, grant.AccessGrantPayload().Audience.Interface)
	require.Equal(t, f.appPrincipal, grant.AccessGrantPayload().Scope.GetPrincipalOwned().Owner)

}

func TestApplicationEnrollmentTicketIsOneUseUnderConcurrencyAndBoundToAudience(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	wrong := request
	wrong.Challenge.Binding.Audience.Interface = identityprotocol.Interface_INTERFACE_OPERATOR
	_, err := f.service.EnrollApplication(f.ctx, f.appBinding, wrong)
	require.ErrorIs(t, err, ErrInvalidArgument)

	var successes atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, enrollErr := f.service.EnrollApplication(f.ctx, f.appBinding, request)
			if enrollErr == nil {
				successes.Add(1)
			} else {
				require.Error(t, enrollErr)
			}
		}()
	}
	wait.Wait()
	require.Equal(t, int32(1), successes.Load())
}

func TestApplicationEnrollmentTicketExpiryIsHalfOpenAndSecretsAreRedacted(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	f.clock.Advance(identitycontract.ApplicationEnrollmentTicketLifetime)
	_, err := f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnauthenticated)
	f.clock.Advance(-time.Second)
	_, err = f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnauthenticated)

	raw, jsonErr := json.Marshal(ticket.Ticket)
	require.NoError(t, jsonErr)
	formatted := fmt.Sprintf("%v %#v %x %s", ticket.Ticket, ticket.Ticket, ticket.Ticket, raw)
	require.NotContains(t, formatted, fmt.Sprintf("%x", ticket.Ticket[:]))
}

func TestApplicationEnrollmentTransactionRollbackLeavesTicketUsable(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	wrapped := &rollbackAfterUpdateDatabase{Database: f.database}
	wrapped.fail.Store(true)
	f.service.grants.database = wrapped
	_, err := f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = f.service.enrollments.load(f.ctx, f.nodeID, f.appPrincipal)
	require.Error(t, err)

	wrapped.fail.Store(false)
	result, err := f.service.EnrollApplication(f.ctx, f.appBinding, f.enrollmentRequest(ticket.Ticket))
	require.NoError(t, err)
	require.Equal(t, f.appPrincipal, result.Principal)
}

func TestApplicationEnrollmentTicketSurvivesRestartButProofDoesNot(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	restarted, err := NewService(Config{Database: f.database, Clock: f.clock, Entropy: &sequentialEntropy{next: 0xd1}, EnableApplicationEnrollment: true, GrantIssuer: testAccessGrantIssuer{key: f.node}})
	require.NoError(t, err)
	f.service = restarted
	_, err = f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnauthenticated)
	result, err := f.service.EnrollApplication(f.ctx, f.appBinding, f.enrollmentRequest(ticket.Ticket))
	require.NoError(t, err)
	require.Equal(t, f.appPrincipal, result.Principal)
}

func TestCorruptApplicationEnrollmentTicketFailsClosed(t *testing.T) {
	f := newApplicationEnrollmentFixture(t)
	ticket := f.issueTicket()
	request := f.enrollmentRequest(ticket.Ticket)
	key := applicationEnrollmentTicketKey(f.nodeID, f.appPrincipal)
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		raw, found, err := tx.Get(applicationEnrollmentTicketsBucket, key)
		if err != nil || !found {
			return err
		}
		raw[len(raw)-1] ^= 0xff
		return tx.Put(applicationEnrollmentTicketsBucket, key, raw)
	}))
	_, err := f.service.EnrollApplication(f.ctx, f.appBinding, request)
	require.ErrorIs(t, err, ErrUnavailable)
}
