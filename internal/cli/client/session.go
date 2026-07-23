package client

import (
	"context"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const operatorSessionScheme = "ArdentsOperatorSession"

var ErrInvalidAuthenticationResponse = errors.New("invalid authentication response")
var ErrSessionInvalidated = errors.New("Principal session login was invalidated")
var ErrSessionSignerUnavailable = errors.New("Principal session signer is unavailable")

// SessionSigner is intentionally typed: routine CLI authentication cannot ask
// it to sign opaque bytes or load the Principal root key.
type SessionSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*identityaccess.Artifact, error)
	SignAuthenticationChallenge(context.Context, identityaccess.Challenge) ([]byte, error)
}

type authenticationService interface {
	BeginAuthentication(context.Context, *connect.Request[ardentsv1.BeginAuthenticationRequest]) (*connect.Response[ardentsv1.BeginAuthenticationResponse], error)
	CompleteAuthentication(context.Context, *connect.Request[ardentsv1.CompleteAuthenticationRequest]) (*connect.Response[ardentsv1.CompleteAuthenticationResponse], error)
	EndSession(context.Context, *connect.Request[ardentsv1.EndSessionRequest]) (*connect.Response[ardentsv1.EndSessionResponse], error)
}

// SessionKey is the complete version-1 cache identity. It deliberately omits
// transport peer material because a manager is scoped to one effective local
// socket or one managed SSH tunnel and is never shared across transports.
type SessionKey struct {
	NodePrincipal   string
	Interface       identityprotocol.Interface
	ProtocolMajor   uint32
	SignerPrincipal string
}

type cachedSession struct {
	secret     identityaccess.SessionSecret
	expiresAt  time.Time
	generation uint64
}

type loginFlight struct {
	done  chan struct{}
	err   error
	epoch uint64
}

// SessionManager owns process-local Operator sessions for one effective
// transport. Session secrets are never persisted or exposed by Status.
type SessionManager struct {
	auth       authenticationService
	signer     SessionSigner
	targetNode string
	now        func() time.Time

	mu      sync.Mutex
	entries map[SessionKey]cachedSession
	flights map[SessionKey]*loginFlight
	nextGen uint64
	active  SessionKey
	epoch   uint64
}

func NewSessionManager(auth authenticationService, signer SessionSigner, targetNode string, now func() time.Time) *SessionManager {
	if now == nil {
		now = time.Now
	}
	return &SessionManager{auth: auth, signer: signer, targetNode: strings.TrimSpace(targetNode), now: now, entries: make(map[SessionKey]cachedSession), flights: make(map[SessionKey]*loginFlight)}
}

func (m *SessionManager) key(ctx context.Context) (SessionKey, error) {
	if m == nil || m.auth == nil || m.signer == nil || m.targetNode == "" {
		return SessionKey{}, ErrInvalidAuthenticationResponse
	}
	principal, err := m.signer.Principal(ctx)
	if err != nil {
		return SessionKey{}, signerFailure(ctx)
	}
	principal = strings.TrimSpace(principal)
	if principal == "" {
		return SessionKey{}, ErrInvalidAuthenticationResponse
	}
	return SessionKey{NodePrincipal: m.targetNode, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor, SignerPrincipal: principal}, nil
}

func (m *SessionManager) authorization(ctx context.Context) (string, uint64, error) {
	key, err := m.key(ctx)
	if err != nil {
		return "", 0, err
	}
	for {
		now := m.now().UTC()
		m.mu.Lock()
		if entry, ok := m.entries[key]; ok {
			if now.Before(entry.expiresAt) {
				m.active = key
				header := operatorSessionScheme + " " + base64.RawURLEncoding.EncodeToString(entry.secret[:])
				m.mu.Unlock()
				return header, entry.generation, nil
			}
			m.zeroAndDelete(key, entry)
		}
		if flight, ok := m.flights[key]; ok {
			done := flight.done
			m.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-done:
				if flight.err != nil {
					return "", 0, flight.err
				}
				continue
			}
		}
		flight := &loginFlight{done: make(chan struct{}), epoch: m.epoch}
		m.flights[key] = flight
		m.mu.Unlock()

		entry, loginErr := m.login(ctx, key, now)
		m.mu.Lock()
		if loginErr == nil && flight.epoch != m.epoch {
			zeroSession(&entry)
			loginErr = ErrSessionInvalidated
		}
		if loginErr == nil {
			m.nextGen++
			entry.generation = m.nextGen
			m.entries[key] = entry
			m.active = key
		}
		flight.err = loginErr
		delete(m.flights, key)
		close(flight.done)
		m.mu.Unlock()
		if loginErr != nil {
			return "", 0, loginErr
		}
	}
}

func (m *SessionManager) login(ctx context.Context, key SessionKey, now time.Time) (cachedSession, error) {
	begin, err := m.auth.BeginAuthentication(ctx, connect.NewRequest(&ardentsv1.BeginAuthenticationRequest{
		PrincipalId: key.SignerPrincipal,
		Purpose:     identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
	}))
	if err != nil {
		return cachedSession{}, err
	}
	if begin == nil || begin.Msg == nil || messageHasUnknown(begin.Msg.ProtoReflect()) {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	challenge, err := identityaccess.ParseChallengeFields(begin.Msg.GetChallenge())
	if err != nil || challenge.Principal != key.SignerPrincipal || challenge.Binding.Audience.Node != key.NodePrincipal || challenge.Binding.Audience.Interface != key.Interface || challenge.Binding.Audience.ProtocolMajor != key.ProtocolMajor || challenge.Binding.TransportProfile != identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1 || challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION || challenge.IssuedAt.After(now) || !now.Before(challenge.ExpiresAt) || isZeroPeer(challenge.Binding.PeerBinding) {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	credential, err := m.signer.Credential(ctx)
	if err != nil {
		return cachedSession{}, signerFailure(ctx)
	}
	payload := credential.KeyCredentialPayload()
	if payload == nil || payload.Subject != key.SignerPrincipal || len(payload.RootPublicKey) != ed25519.PublicKeySize {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	credentialRaw, err := credential.MarshalBinary()
	if err != nil {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	signature, err := m.signer.SignAuthenticationChallenge(ctx, challenge)
	if err != nil {
		return cachedSession{}, signerFailure(ctx)
	}
	if len(signature) != ed25519.SignatureSize {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	complete, err := m.auth.CompleteAuthentication(ctx, connect.NewRequest(&ardentsv1.CompleteAuthenticationRequest{
		ChallengeId: append([]byte(nil), challenge.ID[:]...), PrincipalId: key.SignerPrincipal,
		RootPublicKey: append([]byte(nil), payload.RootPublicKey...), Credential: credentialRaw, Signature: append([]byte(nil), signature...),
	}))
	if err != nil {
		return cachedSession{}, err
	}
	return validateCompleteResponse(complete.Msg, now)
}

func validateCompleteResponse(response *ardentsv1.CompleteAuthenticationResponse, now time.Time) (cachedSession, error) {
	if response == nil || messageHasUnknown(response.ProtoReflect()) || len(response.SessionSecret) != identitycontract.SessionSecretBytes || len(response.EnrollmentProof) != 0 || !validSessionID(response.SessionId) || response.ExpiresAt == nil || !response.ExpiresAt.IsValid() || response.ExpiresAt.Nanos != 0 || response.ExpiresAt.Seconds < identitycontract.LowerTimestampUnix || response.ExpiresAt.Seconds >= identitycontract.UpperTimestampUnix {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	expiresAt := response.ExpiresAt.AsTime()
	if !now.Before(expiresAt) || expiresAt.After(now.Add(identitycontract.MaxSessionLifetime)) {
		return cachedSession{}, ErrInvalidAuthenticationResponse
	}
	var secret identityaccess.SessionSecret
	copy(secret[:], response.SessionSecret)
	return cachedSession{secret: secret, expiresAt: expiresAt}, nil
}

func validSessionID(value string) bool {
	if len(value) != 55 || !strings.HasPrefix(value, "s1_") {
		return false
	}
	suffix := value[3:]
	if suffix != strings.ToLower(suffix) {
		return false
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	raw, err := encoding.DecodeString(strings.ToUpper(suffix))
	return err == nil && len(raw) == 32 && strings.ToLower(encoding.EncodeToString(raw)) == suffix
}

func isZeroPeer(peer [identitycontract.PeerBindingBytes]byte) bool {
	return peer == [identitycontract.PeerBindingBytes]byte{}
}

func signerFailure(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrSessionSignerUnavailable
}

func messageHasUnknown(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len() && !found; i++ {
				found = messageHasUnknown(list.Get(i).Message())
			}
		} else if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				found = messageHasUnknown(item.Message())
				return !found
			})
		} else if field.Message() != nil {
			found = messageHasUnknown(value.Message())
		}
		return !found
	})
	return found
}

func (m *SessionManager) evictGeneration(generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, entry := range m.entries {
		if entry.generation == generation {
			m.zeroAndDelete(key, entry)
			if m.active == key {
				m.active = SessionKey{}
			}
			return
		}
	}
}

func (m *SessionManager) Status() SessionKey {
	if m == nil {
		return SessionKey{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[m.active]
	if !ok {
		return SessionKey{}
	}
	if !m.now().UTC().Before(entry.expiresAt) {
		key := m.active
		m.zeroAndDelete(key, entry)
		m.active = SessionKey{}
		return SessionKey{}
	}
	return m.active
}

func (m *SessionManager) Logout() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.epoch++
	secrets := make([]identityaccess.SessionSecret, 0, len(m.entries))
	for key, entry := range m.entries {
		secrets = append(secrets, entry.secret)
		m.zeroAndDelete(key, entry)
	}
	m.active = SessionKey{}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for index := range secrets {
		request := connect.NewRequest(&ardentsv1.EndSessionRequest{})
		request.Header().Set("Authorization", operatorSessionScheme+" "+base64.RawURLEncoding.EncodeToString(secrets[index][:]))
		_, _ = m.auth.EndSession(ctx, request)
		clear(secrets[index][:])
	}
}

func zeroSession(entry *cachedSession) {
	for i := range entry.secret {
		entry.secret[i] = 0
	}
}

func (m *SessionManager) zeroAndDelete(key SessionKey, entry cachedSession) {
	zeroSession(&entry)
	m.entries[key] = entry
	delete(m.entries, key)
}

type sessionInterceptor struct{ manager *SessionManager }

func newSessionInterceptor(manager *SessionManager) *sessionInterceptor {
	return &sessionInterceptor{manager: manager}
}

func (i *sessionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if request.Header().Get("Authorization") != "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("multiple authentication schemes are forbidden"))
		}
		header, generation, err := i.manager.authorization(ctx)
		if err != nil {
			return nil, err
		}
		request.Header().Set("Authorization", header)
		response, err := next(ctx, request)
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			return response, err
		}
		i.manager.evictGeneration(generation)
		header, _, authErr := i.manager.authorization(ctx)
		if authErr != nil {
			return nil, authErr
		}
		request.Header().Set("Authorization", header)
		return next(ctx, request)
	}
}

func (i *sessionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		connection := next(ctx, spec)
		if connection.RequestHeader().Get("Authorization") != "" {
			return &sessionStreamingConn{StreamingClientConn: connection, initErr: connect.NewError(connect.CodeUnauthenticated, errors.New("multiple authentication schemes are forbidden"))}
		}
		header, generation, err := i.manager.authorization(ctx)
		wrapped := &sessionStreamingConn{StreamingClientConn: connection, ctx: ctx, spec: spec, next: next, manager: i.manager, generation: generation, initErr: err}
		if err == nil {
			connection.RequestHeader().Set("Authorization", header)
		}
		return wrapped
	}
}

func (i *sessionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type sessionStreamingConn struct {
	connect.StreamingClientConn
	ctx        context.Context
	spec       connect.Spec
	next       connect.StreamingClientFunc
	manager    *SessionManager
	generation uint64
	initErr    error

	mu            sync.Mutex
	sent          proto.Message
	requestClosed bool
	received      bool
	refreshed     bool
}

func (c *sessionStreamingConn) Send(message any) error {
	if c.initErr != nil {
		return c.initErr
	}
	if wire, ok := message.(proto.Message); ok {
		c.mu.Lock()
		if c.sent == nil {
			c.sent = proto.Clone(wire)
		}
		c.mu.Unlock()
	}
	return c.StreamingClientConn.Send(message)
}

func (c *sessionStreamingConn) CloseRequest() error {
	if c.initErr != nil {
		return c.initErr
	}
	c.mu.Lock()
	c.requestClosed = true
	c.mu.Unlock()
	return c.StreamingClientConn.CloseRequest()
}

func (c *sessionStreamingConn) Receive(message any) error {
	if c.initErr != nil {
		return c.initErr
	}
	err := c.StreamingClientConn.Receive(message)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.received = true
		return nil
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated || c.received || c.refreshed || c.sent == nil {
		return err
	}
	c.refreshed = true
	c.manager.evictGeneration(c.generation)
	header, generation, authErr := c.manager.authorization(c.ctx)
	if authErr != nil {
		return authErr
	}
	fresh := c.next(c.ctx, c.spec)
	if fresh.RequestHeader().Get("Authorization") != "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("multiple authentication schemes are forbidden"))
	}
	fresh.RequestHeader().Set("Authorization", header)
	if sendErr := fresh.Send(proto.Clone(c.sent)); sendErr != nil {
		return sendErr
	}
	if c.requestClosed {
		if closeErr := fresh.CloseRequest(); closeErr != nil {
			return closeErr
		}
	}
	c.StreamingClientConn = fresh
	c.generation = generation
	receiveErr := fresh.Receive(message)
	if receiveErr == nil {
		c.received = true
	}
	return receiveErr
}

func (c *sessionStreamingConn) CloseResponse() error {
	if c.initErr != nil {
		return c.initErr
	}
	return c.StreamingClientConn.CloseResponse()
}
