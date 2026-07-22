package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type fakeAccessClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeAccessClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeAccessClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type sequentialEntropy struct {
	mu   sync.Mutex
	next byte
}

type viewGateDatabase struct {
	storage.Database
	once    sync.Once
	viewed  chan struct{}
	release chan struct{}
}

func (d *viewGateDatabase) View(ctx context.Context, callback func(storage.ReadTransaction) error) error {
	err := d.Database.View(ctx, callback)
	d.once.Do(func() { close(d.viewed); <-d.release })
	return err
}

func (r *sequentialEntropy) Read(destination []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range destination {
		destination[index] = r.next
		r.next++
	}
	return len(destination), nil
}

type serviceFixture struct {
	t                           *testing.T
	ctx                         context.Context
	clock                       *fakeAccessClock
	database                    *storage.Handle
	service                     *Service
	root, device, node          ed25519.PrivateKey
	principal, nodeID, deviceID string
	binding                     AuthenticationBinding
	source                      SourceKey
	credential                  *Artifact
	dir                         string
}

func newServiceFixture(t *testing.T) *serviceFixture {
	t.Helper()
	ctx := context.Background()
	clock := &fakeAccessClock{now: time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC)}
	dir := t.TempDir()
	database, err := storage.OpenIdentityAccess(ctx, dir, StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(ctx)) })
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, 32))
	principal, _ := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	nodePrincipal, _ := identityprincipal.FromEd25519PublicKey(node.Public().(ed25519.PublicKey))
	deviceID, _ := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	credential, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(clock.now.Add(-time.Hour)), NotAfter: timestamppb.New(clock.now.Add(time.Hour))}, root)
	require.NoError(t, err)
	var peer [32]byte
	copy(peer[:], bytes.Repeat([]byte{0x44}, 32))
	var source SourceKey
	copy(source[:], bytes.Repeat([]byte{0x55}, 32))
	binding := AuthenticationBinding{Audience: Audience{Node: nodePrincipal.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer}
	service, err := NewService(Config{Database: database, Clock: clock, Entropy: &sequentialEntropy{next: 1}})
	require.NoError(t, err)
	return &serviceFixture{t: t, ctx: ctx, clock: clock, database: database, service: service, root: root, device: device, node: node, principal: principal.String(), nodeID: nodePrincipal.String(), deviceID: deviceID.String(), binding: binding, source: source, credential: credential, dir: dir}
}

func (f *serviceFixture) begin(purpose identityprotocol.ChallengePurpose) Challenge {
	challenge, err := f.service.Begin(f.ctx, BeginRequest{Principal: f.principal, Purpose: purpose, Binding: f.binding, Source: f.source})
	require.NoError(f.t, err)
	return challenge
}
func (f *serviceFixture) sessionRequest(challenge Challenge) CompleteRequest {
	raw, err := f.credential.MarshalBinary()
	require.NoError(f.t, err)
	signed, err := challengeSigningBytes(challenge)
	require.NoError(f.t, err)
	var root [32]byte
	copy(root[:], f.root.Public().(ed25519.PublicKey))
	return CompleteRequest{ChallengeID: challenge.ID, Principal: f.principal, Binding: f.binding, Source: f.source, RootPublicKey: root, Credential: raw, Signature: ed25519.Sign(f.device, signed)}
}

func TestServiceSessionHappyPathBindsCredentialAndStoresOnlyHMAC(t *testing.T) {
	f := newServiceFixture(t)
	challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
	result, err := f.service.Complete(f.ctx, f.sessionRequest(challenge))
	require.NoError(t, err)
	require.NotNil(t, result.Session)
	require.NotNil(t, result.SessionSecret)
	require.Nil(t, result.EnrollmentProof)
	require.Equal(t, f.principal, result.Session.Principal)
	require.Equal(t, f.deviceID, result.Session.DeviceID)
	require.Equal(t, f.credential.ID(), result.Session.CredentialID)
	require.Equal(t, f.binding, result.Session.Binding)
	require.Equal(t, identitycontract.DefaultSessionLifetime, result.Session.ExpiresAt.Sub(result.Session.IssuedAt))
	session, err := f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.NoError(t, err)
	require.Equal(t, result.Session.ID, session.ID)
	wrongBinding := f.binding
	wrongBinding.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, wrongBinding)
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.NoError(t, err)
	lookup := f.service.sessions.lookup(*result.SessionSecret)
	require.Contains(t, f.service.sessions.items, lookup)
	require.NotEqual(t, [32]byte(*result.SessionSecret), lookup)
	require.NotContains(t, fmt.Sprintf("%v %#v %x", result, result.SessionSecret, *result.SessionSecret), fmt.Sprintf("%x", result.SessionSecret[:]))
}

func TestServiceCompleteIsAtomicSingleUseUnderConcurrency(t *testing.T) {
	f := newServiceFixture(t)
	request := f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION))
	var success atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < 100; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := f.service.Complete(f.ctx, request)
			if err == nil {
				success.Add(1)
			} else {
				require.ErrorIs(t, err, ErrUnauthenticated)
			}
		}()
	}
	wait.Wait()
	require.Equal(t, int32(1), success.Load())
}

func TestServiceRejectsCrossBindingAndExpiryWithoutFallback(t *testing.T) {
	for name, mutate := range map[string]func(*CompleteRequest){
		"node": func(r *CompleteRequest) { r.Binding.Audience.Node = fakePrincipal(0x91) },
		"interface": func(r *CompleteRequest) {
			r.Binding.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
		},
		"peer":   func(r *CompleteRequest) { r.Binding.PeerBinding[0] ^= 1 },
		"source": func(r *CompleteRequest) { r.Source[0] ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			f := newServiceFixture(t)
			r := f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION))
			mutate(&r)
			_, err := f.service.Complete(f.ctx, r)
			require.ErrorIs(t, err, ErrUnauthenticated)
		})
	}
	f := newServiceFixture(t)
	r := f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION))
	f.clock.Advance(identitycontract.ChallengeLifetime)
	_, err := f.service.Complete(f.ctx, r)
	require.ErrorIs(t, err, ErrUnauthenticated)
	f.clock.Advance(-time.Second)
	_, err = f.service.Complete(f.ctx, r)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestServiceRejectsWrongDomainKeyAndMalformedCredentialUniformly(t *testing.T) {
	for name, mutate := range map[string]func(*serviceFixture, Challenge, *CompleteRequest){
		"wrong_domain": func(f *serviceFixture, c Challenge, r *CompleteRequest) {
			c.Purpose = identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
			signed, _ := challengeSigningBytes(c)
			r.Signature = ed25519.Sign(f.device, signed)
		},
		"wrong_root": func(_ *serviceFixture, _ Challenge, r *CompleteRequest) {
			copy(r.RootPublicKey[:], bytes.Repeat([]byte{0x99}, 32))
		},
		"malformed_credential": func(_ *serviceFixture, _ Challenge, r *CompleteRequest) { r.Credential = []byte{0xff, 0x00} },
		"wrong_device_key": func(_ *serviceFixture, c Challenge, r *CompleteRequest) {
			signed, _ := challengeSigningBytes(c)
			wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x77}, 32))
			r.Signature = ed25519.Sign(wrong, signed)
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newServiceFixture(t)
			challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
			request := f.sessionRequest(challenge)
			mutate(f, challenge, &request)
			_, err := f.service.Complete(f.ctx, request)
			require.Equal(t, ErrUnauthenticated, err)
		})
	}
}

func TestServiceEnrollmentProofNeverIssuesSessionAndIsOneUse(t *testing.T) {
	f := newServiceFixture(t)
	challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF)
	signed, err := challengeSigningBytes(challenge)
	require.NoError(t, err)
	var root [32]byte
	copy(root[:], f.root.Public().(ed25519.PublicKey))
	result, err := f.service.Complete(f.ctx, CompleteRequest{ChallengeID: challenge.ID, Principal: f.principal, Binding: f.binding, Source: f.source, RootPublicKey: root, Signature: ed25519.Sign(f.root, signed)})
	require.NoError(t, err)
	require.Nil(t, result.Session)
	require.Nil(t, result.SessionSecret)
	require.NotNil(t, result.EnrollmentProof)
	require.True(t, f.service.consumeEnrollmentProof(*result.EnrollmentProof, challenge))
	require.False(t, f.service.consumeEnrollmentProof(*result.EnrollmentProof, challenge))
	challenge = f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF)
	request := f.sessionRequest(challenge)
	_, err = f.service.Complete(f.ctx, request)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestServiceDeviceRevocationIsDurablePreemptiveAndInvalidatesRenewedCredentials(t *testing.T) {
	f := newServiceFixture(t)
	result, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(t, err)
	revocation, err := SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: f.deviceID, Issuer: f.nodeID, Audience: &identityprotocol.Audience{Node: f.nodeID, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, RevokedAt: timestamppb.New(f.clock.Now()), TargetDeviceId: f.deviceID, Subject: f.principal}, f.node, f.clock.Now())
	require.NoError(t, err)
	require.NoError(t, f.service.recordDeviceRevocation(f.ctx, revocation))
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.ErrorIs(t, err, ErrUnauthenticated)
	restarted, err := NewService(Config{Database: f.database, Clock: f.clock, Entropy: &sequentialEntropy{next: 151}})
	require.NoError(t, err)
	_, err = restarted.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
	f.service = restarted
	_, err = f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestServicePreemptiveRevocationIsNodeLocal(t *testing.T) {
	f := newServiceFixture(t)
	revocation, err := SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: f.deviceID, Issuer: f.nodeID, Audience: &identityprotocol.Audience{Node: f.nodeID, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, RevokedAt: timestamppb.New(f.clock.Now()), TargetDeviceId: f.deviceID, Subject: f.principal}, f.node, f.clock.Now())
	require.NoError(t, err)
	require.NoError(t, f.service.recordDeviceRevocation(f.ctx, revocation))
	_, err = f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.ErrorIs(t, err, ErrUnauthenticated)
	otherNode := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x66}, 32))
	otherID, _ := identityprincipal.FromEd25519PublicKey(otherNode.Public().(ed25519.PublicKey))
	f.binding.Audience.Node = otherID.String()
	result, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(t, err)
	require.Equal(t, otherID.String(), result.Session.Binding.Audience.Node)
}

func TestCompleteAndDeviceRevocationShareLinearizationBoundary(t *testing.T) {
	f := newServiceFixture(t)
	gate := &viewGateDatabase{Database: f.database, viewed: make(chan struct{}), release: make(chan struct{})}
	service, err := NewService(Config{Database: gate, Clock: f.clock, Entropy: &sequentialEntropy{next: 31}})
	require.NoError(t, err)
	f.service = service
	challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
	request := f.sessionRequest(challenge)
	revocation, err := SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: f.deviceID, Issuer: f.nodeID, Audience: &identityprotocol.Audience{Node: f.nodeID, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, RevokedAt: timestamppb.New(f.clock.Now()), TargetDeviceId: f.deviceID, Subject: f.principal}, f.node, f.clock.Now())
	require.NoError(t, err)
	completeDone := make(chan struct {
		result CompleteResult
		err    error
	}, 1)
	go func() {
		result, completeErr := service.Complete(f.ctx, request)
		completeDone <- struct {
			result CompleteResult
			err    error
		}{result, completeErr}
	}()
	<-gate.viewed
	revokeDone := make(chan error, 1)
	go func() { revokeDone <- service.recordDeviceRevocation(f.ctx, revocation) }()
	select {
	case err := <-revokeDone:
		t.Fatalf("revocation crossed active Complete boundary: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(gate.release)
	completed := <-completeDone
	require.NoError(t, completed.err)
	require.NotNil(t, completed.result.SessionSecret)
	require.NoError(t, <-revokeDone)
	_, err = service.AuthenticateSession(f.ctx, *completed.result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)

	g := newServiceFixture(t)
	revocation, err = SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{Version: 1, TargetId: g.deviceID, Issuer: g.nodeID, Audience: &identityprotocol.Audience{Node: g.nodeID, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, RevokedAt: timestamppb.New(g.clock.Now()), TargetDeviceId: g.deviceID, Subject: g.principal}, g.node, g.clock.Now())
	require.NoError(t, err)
	require.NoError(t, g.service.recordDeviceRevocation(g.ctx, revocation))
	_, err = g.service.Complete(g.ctx, g.sessionRequest(g.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestServiceRestartDropsSessionsButCredentialStillAuthenticates(t *testing.T) {
	f := newServiceFixture(t)
	result, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(t, err)
	restarted, err := NewService(Config{Database: f.database, Clock: f.clock, Entropy: &sequentialEntropy{next: 177}})
	require.NoError(t, err)
	_, err = restarted.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
	f.service = restarted
	next, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(t, err)
	require.NotNil(t, next.Session)
	var persisted int
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		return tx.ForEach(deviceRevocationsBucket, func(_, _ []byte) error { persisted++; return nil })
	}))
	require.Zero(t, persisted)
}

func TestServiceSessionExpiryIsHalfOpenAndNeverRevives(t *testing.T) {
	f := newServiceFixture(t)
	result, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(t, err)
	f.clock.Advance(identitycontract.DefaultSessionLifetime - time.Second)
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.NoError(t, err)
	f.clock.Advance(time.Second)
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
	f.clock.Advance(-time.Second)
	_, err = f.service.AuthenticateSession(f.ctx, *result.SessionSecret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestServiceBoundsRateCapacityAndSessionGroup(t *testing.T) {
	f := newServiceFixture(t)
	for i := 0; i < identitycontract.BeginRateBurst; i++ {
		_, err := f.service.Begin(f.ctx, BeginRequest{Principal: f.principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, Binding: f.binding, Source: f.source})
		require.NoError(t, err)
	}
	_, err := f.service.Begin(f.ctx, BeginRequest{Principal: f.principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, Binding: f.binding, Source: f.source})
	require.ErrorIs(t, err, ErrResourceExhausted)
	f.clock.Advance(identitycontract.ChallengeLifetime)
	_, err = f.service.Begin(f.ctx, BeginRequest{Principal: f.principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, Binding: f.binding, Source: f.source})
	require.NoError(t, err)

	g := newServiceFixture(t)
	for i := 0; i < identitycontract.MaxActiveSessionsPerSourceKey; i++ {
		result, completeErr := g.service.Complete(g.ctx, g.sessionRequest(g.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
		require.NoError(t, completeErr)
		require.NotNil(t, result.Session)
		g.clock.Advance(6 * time.Second)
	}
	_, err = g.service.Complete(g.ctx, g.sessionRequest(g.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.ErrorIs(t, err, ErrResourceExhausted)
}

func TestServiceUniformErrorsAndRedactedAudit(t *testing.T) {
	f := newServiceFixture(t)
	var events []AuditEvent
	f.service.audit = AuditSinkFunc(func(event AuditEvent) { events = append(events, event) })
	request := f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION))
	request.Signature[0] ^= 1
	_, err := f.service.Complete(f.ctx, request)
	require.Equal(t, ErrUnauthenticated, err)
	require.NotEmpty(t, events)
	text := fmt.Sprintf("%v", events)
	require.NotContains(t, text, fmt.Sprintf("%x", request.Signature))
	require.NotContains(t, text, fmt.Sprintf("%x", request.Credential))
	require.NotContains(t, text, fmt.Sprintf("%x", request.ChallengeID))
}

func TestAuthenticationSecretsAndNonceAreRedactedFromFormattingAndJSON(t *testing.T) {
	f := newServiceFixture(t)
	challenge := f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
	result, err := f.service.Complete(f.ctx, f.sessionRequest(challenge))
	require.NoError(t, err)
	for _, value := range []any{challenge, *result.SessionSecret, CompleteResult{Session: result.Session, SessionSecret: result.SessionSecret}} {
		formatted := fmt.Sprintf("%v %#v %x", value, value, value)
		encoded, jsonErr := json.Marshal(value)
		require.NoError(t, jsonErr)
		combined := formatted + string(encoded)
		require.NotContains(t, combined, hex.EncodeToString(challenge.Nonce[:]))
		require.NotContains(t, combined, hex.EncodeToString(result.SessionSecret[:]))
		require.NotContains(t, combined, fmt.Sprintf("%v", challenge.Nonce))
	}
}

func fakePrincipal(seed byte) string {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 32))
	id, _ := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	return id.String()
}

func TestStorageSchemaIsOneFreshPreReleaseSchema(t *testing.T) {
	schema := StorageSchema()
	require.Equal(t, uint32(1), schema.Version)
	require.Len(t, schema.Migrations, 1)
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), schema)
	require.NoError(t, err)
	require.NoError(t, database.Close(context.Background()))
}

func TestServiceSessionLifetimeConfigurationBoundaries(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), StorageSchema())
	require.NoError(t, err)
	defer database.Close(context.Background())
	_, err = NewService(Config{Database: database, Entropy: &sequentialEntropy{}, SessionLifetime: identitycontract.MaxSessionLifetime})
	require.NoError(t, err)
	_, err = NewService(Config{Database: database, Entropy: &sequentialEntropy{}, SessionLifetime: identitycontract.MaxSessionLifetime + time.Second})
	require.Error(t, err)
	_, err = NewService(Config{Database: database, Entropy: &sequentialEntropy{}, SessionLifetime: -time.Second})
	require.Error(t, err)
}

func TestAuthenticationErrorsDoNotWrapInternalReasons(t *testing.T) {
	require.False(t, errors.Is(ErrUnauthenticated, errInvalid))
}

func TestChallengeCanonicalVector(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	principal, _ := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	var id ChallengeID
	for index := range id {
		id[index] = byte(index + 1)
	}
	var nonce, peer [32]byte
	for index := range nonce {
		nonce[index] = byte(0x20 + index)
		peer[index] = byte(0x80 + index)
	}
	challenge := Challenge{Version: 1, ID: id, Nonce: nonce, Principal: principal.String(), Binding: AuthenticationBinding{Audience: Audience{Node: fakePrincipal(0x33), Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer}, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, IssuedAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), ExpiresAt: time.Date(2030, 1, 2, 3, 6, 5, 0, time.UTC)}
	raw, err := challengeSigningBytes(challenge)
	require.NoError(t, err)
	require.Equal(t, "4e02e35c5d54243e4a6829e60f861948bf61aab5ccf39f5d40a05e96f82349895eb9e2d14c4aae8cbddd67add1cdcbacf8b8e5c264daa956c61760ab64a45d0f", hex.EncodeToString(ed25519.Sign(device, raw)))
	challenge.Purpose = identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	raw, err = challengeSigningBytes(challenge)
	require.NoError(t, err)
	require.Equal(t, "14645769955b79f02d99fe9f528d9bc779ba81eb168dd044596f4e15ae0897e466a3bc483c8825d71c52e9e6ffcf36f4125ac4ab2143105f20e57fb023b91300", hex.EncodeToString(ed25519.Sign(root, raw)))
}
