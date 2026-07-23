package adapter

import (
	"context"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkerrors "ardents/sdk/go/errors"
	sdkidentity "ardents/sdk/go/identity"
	applicationidentityv1 "ardents/sdk/go/protocol/applicationidentityv1"
	applicationidentityv1connect "ardents/sdk/go/protocol/applicationidentityv1/applicationv1connect"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	applicationSessionScheme    = "ArdentsApplicationSession"
	applicationDelegationHeader = "Ardents-Delegation"
)

type SessionSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*sdkidentity.Artifact, error)
	SignAuthenticationChallenge(context.Context, sdkidentity.Challenge) ([]byte, error)
}

type authenticationService interface {
	BeginAuthentication(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error)
	CompleteAuthentication(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error)
	EndSession(context.Context, *connect.Request[applicationidentityv1.EndSessionRequest]) (*connect.Response[applicationidentityv1.EndSessionResponse], error)
}

type SessionStatus struct {
	Authenticated   bool
	NodePrincipal   string
	SignerPrincipal string
}

type sessionKey struct {
	node      string
	principal string
}

type cachedSession struct {
	secret     [identitycontract.SessionSecretBytes]byte
	expiresAt  time.Time
	generation uint64
}

type loginFlight struct {
	done  chan struct{}
	err   error
	epoch uint64
}

// SessionManager owns process-local Application sessions for one effective
// protected Unix socket.
type SessionManager struct {
	auth       authenticationService
	signer     SessionSigner
	targetNode string
	now        func() time.Time

	mu      sync.Mutex
	entries map[sessionKey]cachedSession
	flights map[sessionKey]*loginFlight
	nextGen uint64
	active  sessionKey
	epoch   uint64
}

func NewSessionManager(httpClient connect.HTTPClient, endpoint string, signer SessionSigner, targetNode string, now func() time.Time) *SessionManager {
	if now == nil {
		now = time.Now
	}
	auth := applicationidentityv1connect.NewIdentityServiceClient(
		httpClient,
		strings.TrimRight(endpoint, "/"),
		connect.WithReadMaxBytes(identitycontract.MaxArtifactBytes+4<<10),
		connect.WithSendMaxBytes(identitycontract.MaxArtifactBytes+4<<10),
	)
	return &SessionManager{
		auth: auth, signer: signer, targetNode: strings.TrimSpace(targetNode), now: now,
		entries: make(map[sessionKey]cachedSession), flights: make(map[sessionKey]*loginFlight),
	}
}

func (m *SessionManager) key(ctx context.Context) (sessionKey, error) {
	if m == nil || m.auth == nil || m.signer == nil || !validDigestID(m.targetNode, "p1_") {
		return sessionKey{}, invalidAuthenticationResponse()
	}
	principal, err := m.signer.Principal(ctx)
	if err != nil {
		return sessionKey{}, signerUnavailable(ctx)
	}
	principal = strings.TrimSpace(principal)
	if !validDigestID(principal, "p1_") {
		return sessionKey{}, invalidAuthenticationResponse()
	}
	return sessionKey{node: m.targetNode, principal: principal}, nil
}

func (m *SessionManager) Authenticate(ctx context.Context) error {
	_, _, _, err := m.authorization(ctx)
	return err
}

func (m *SessionManager) authorization(ctx context.Context) (string, uint64, sessionKey, error) {
	key, err := m.key(ctx)
	if err != nil {
		return "", 0, sessionKey{}, err
	}
	for {
		now := m.now().UTC()
		m.mu.Lock()
		if entry, ok := m.entries[key]; ok {
			if now.Before(entry.expiresAt) {
				m.active = key
				header := applicationSessionScheme + " " + base64.RawURLEncoding.EncodeToString(entry.secret[:])
				m.mu.Unlock()
				return header, entry.generation, key, nil
			}
			m.zeroAndDelete(key, entry)
		}
		if flight, ok := m.flights[key]; ok {
			done := flight.done
			m.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", 0, sessionKey{}, ctx.Err()
			case <-done:
				if flight.err != nil {
					if ctx.Err() == nil && (errors.Is(flight.err, context.Canceled) || errors.Is(flight.err, context.DeadlineExceeded)) {
						continue
					}
					return "", 0, sessionKey{}, flight.err
				}
				continue
			}
		}
		flight := &loginFlight{done: make(chan struct{}), epoch: m.epoch}
		m.flights[key] = flight
		m.mu.Unlock()

		entry, loginErr := m.login(ctx, key)
		m.mu.Lock()
		if loginErr == nil && flight.epoch != m.epoch {
			zeroSession(&entry)
			loginErr = &sdkerrors.Error{Code: sdkerrors.Unauthenticated, Message: "Application session was invalidated"}
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
			return "", 0, sessionKey{}, loginErr
		}
	}
}

func (m *SessionManager) login(ctx context.Context, key sessionKey) (cachedSession, error) {
	begin, err := m.auth.BeginAuthentication(ctx, connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{
		PrincipalId: key.principal,
		Purpose:     identityv1.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
	}))
	if err != nil {
		return cachedSession{}, mapAuthenticationError(err)
	}
	if begin == nil || begin.Msg == nil || messageHasUnknown(begin.Msg.ProtoReflect()) {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	challenge, err := applicationChallenge(begin.Msg.GetChallenge(), m.now().UTC())
	if err != nil || challenge.Principal != key.principal ||
		challenge.Binding.Audience.Node != key.node ||
		challenge.Binding.Audience.Interface != sdkidentity.InterfaceApplication ||
		challenge.Binding.Audience.ProtocolMajor != identitycontract.ProtocolMajor ||
		challenge.Binding.TransportProfile != sdkidentity.TransportUnixLocalV1 ||
		challenge.Purpose != sdkidentity.ChallengeSession {
		return cachedSession{}, invalidAuthenticationResponse()
	}

	credential, err := m.signer.Credential(ctx)
	if err != nil {
		return cachedSession{}, signerUnavailable(ctx)
	}
	if credential == nil {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	parsed := credential.KeyCredential()
	if parsed == nil || parsed.Subject != key.principal || len(parsed.RootPublicKey) != ed25519.PublicKeySize {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	credentialRaw, err := credential.MarshalBinary()
	if err != nil {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	defer clear(credentialRaw)
	signature, err := m.signer.SignAuthenticationChallenge(ctx, challenge)
	if err != nil {
		return cachedSession{}, signerUnavailable(ctx)
	}
	defer clear(signature)
	if len(signature) != ed25519.SignatureSize {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	completeRequest := &applicationidentityv1.CompleteAuthenticationRequest{
		ChallengeId: append([]byte(nil), challenge.ID[:]...), PrincipalId: key.principal,
		RootPublicKey: append([]byte(nil), parsed.RootPublicKey...), Credential: append([]byte(nil), credentialRaw...),
		Signature: append([]byte(nil), signature...),
	}
	complete, err := m.auth.CompleteAuthentication(ctx, connect.NewRequest(completeRequest))
	clear(completeRequest.Credential)
	clear(completeRequest.Signature)
	if err != nil {
		return cachedSession{}, mapAuthenticationError(err)
	}
	if complete == nil {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	return validateCompleteResponse(complete.Msg, m.now().UTC())
}

func applicationChallenge(wire *identityv1.ChallengeFields, now time.Time) (sdkidentity.Challenge, error) {
	return applicationChallengeForPurpose(wire, now, sdkidentity.ChallengeSession)
}

func validateCompleteResponse(response *applicationidentityv1.CompleteAuthenticationResponse, now time.Time) (cachedSession, error) {
	if response != nil {
		defer clear(response.SessionSecret)
		defer clear(response.EnrollmentProof)
	}
	if response == nil || messageHasUnknown(response.ProtoReflect()) ||
		len(response.SessionSecret) != identitycontract.SessionSecretBytes ||
		len(response.EnrollmentProof) != 0 || !validSessionID(response.SessionId) ||
		response.ExpiresAt == nil || !response.ExpiresAt.IsValid() || response.ExpiresAt.Nanos != 0 ||
		response.ExpiresAt.Seconds < identitycontract.LowerTimestampUnix ||
		response.ExpiresAt.Seconds >= identitycontract.UpperTimestampUnix {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	expiresAt := response.ExpiresAt.AsTime()
	if !now.Before(expiresAt) || expiresAt.After(now.Add(identitycontract.MaxSessionLifetime)) {
		return cachedSession{}, invalidAuthenticationResponse()
	}
	var secret [identitycontract.SessionSecretBytes]byte
	copy(secret[:], response.SessionSecret)
	return cachedSession{secret: secret, expiresAt: expiresAt}, nil
}

func validSessionID(value string) bool {
	return validDigestID(value, "s1_")
}

func validDigestID(value, prefix string) bool {
	if len(value) != len(prefix)+52 || !strings.HasPrefix(value, prefix) {
		return false
	}
	suffix := value[len(prefix):]
	if suffix != strings.ToLower(suffix) {
		return false
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	raw, err := encoding.DecodeString(strings.ToUpper(suffix))
	if err != nil || len(raw) != 32 || strings.ToLower(encoding.EncodeToString(raw)) != suffix {
		return false
	}
	for _, value := range raw {
		if value != 0 {
			return true
		}
	}
	return false
}

// ValidPrincipalID reports whether value is a canonical v1 Principal ID.
func ValidPrincipalID(value string) bool {
	return validDigestID(strings.TrimSpace(value), "p1_")
}

func invalidAuthenticationResponse() error {
	return &sdkerrors.Error{Code: sdkerrors.Internal, Message: "Application authentication response is invalid"}
}

func signerUnavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return &sdkerrors.Error{Code: sdkerrors.Unauthenticated, Message: "Application signer is unavailable"}
}

func mapAuthenticationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return &sdkerrors.Error{Code: connectCode(connectErr.Code()), Message: "Application identity request failed"}
	}
	return &sdkerrors.Error{Code: sdkerrors.Internal, Message: "Application identity request failed"}
}

func messageHasUnknown(message protoreflect.Message) bool {
	if !message.IsValid() || len(message.GetUnknown()) != 0 {
		return true
	}
	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsList() && field.Message() != nil:
			list := value.List()
			for i := 0; i < list.Len() && !found; i++ {
				found = messageHasUnknown(list.Get(i).Message())
			}
		case field.IsMap() && field.MapValue().Message() != nil:
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				found = messageHasUnknown(item.Message())
				return !found
			})
		case field.Message() != nil:
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
				m.active = sessionKey{}
			}
			return
		}
	}
}

func (m *SessionManager) Status() SessionStatus {
	if m == nil {
		return SessionStatus{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[m.active]
	if !ok {
		return SessionStatus{}
	}
	if !m.now().UTC().Before(entry.expiresAt) {
		key := m.active
		m.zeroAndDelete(key, entry)
		m.active = sessionKey{}
		return SessionStatus{}
	}
	return SessionStatus{Authenticated: true, NodePrincipal: m.active.node, SignerPrincipal: m.active.principal}
}

func (m *SessionManager) Logout() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.epoch++
	secrets := make([][identitycontract.SessionSecretBytes]byte, 0, len(m.entries))
	for key, entry := range m.entries {
		secrets = append(secrets, entry.secret)
		m.zeroAndDelete(key, entry)
	}
	m.active = sessionKey{}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for index := range secrets {
		request := connect.NewRequest(&applicationidentityv1.EndSessionRequest{})
		request.Header().Set("Authorization", applicationSessionScheme+" "+base64.RawURLEncoding.EncodeToString(secrets[index][:]))
		_, _ = m.auth.EndSession(ctx, request)
		clear(secrets[index][:])
	}
}

func zeroSession(entry *cachedSession) { clear(entry.secret[:]) }

func (m *SessionManager) zeroAndDelete(key sessionKey, entry cachedSession) {
	zeroSession(&entry)
	m.entries[key] = entry
	delete(m.entries, key)
}

type applicationDelegation struct {
	headerValue string
	node        string
	delegatee   string
}

type SessionInterceptor struct {
	manager    *SessionManager
	delegation *applicationDelegation
}

func NewSessionInterceptor(manager *SessionManager) *SessionInterceptor {
	return &SessionInterceptor{manager: manager}
}

func NewSessionInterceptorWithDelegation(manager *SessionManager, artifact *sdkidentity.Artifact) (*SessionInterceptor, error) {
	if manager == nil || manager.now == nil || artifact == nil || artifact.Kind() != sdkidentity.KindDelegation {
		return nil, invalidApplicationDelegation()
	}
	raw, err := artifact.MarshalBinary()
	if err != nil {
		return nil, invalidApplicationDelegation()
	}
	defer clear(raw)
	if !identitycontract.ValidArtifactSize(len(raw)) {
		return nil, invalidApplicationDelegation()
	}
	parsed, err := sdkidentity.ParseDelegation(raw, manager.now().UTC())
	if err != nil || parsed.ID() != artifact.ID() {
		return nil, invalidApplicationDelegation()
	}
	view := parsed.Delegation()
	if view == nil || view.Audience.Node != manager.targetNode ||
		view.Audience.Interface != sdkidentity.InterfaceApplication ||
		view.Audience.ProtocolMajor != identitycontract.ProtocolMajor ||
		!ValidPrincipalID(view.Delegatee) {
		return nil, invalidApplicationDelegation()
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) == 0 || len(encoded) > base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes) {
		return nil, invalidApplicationDelegation()
	}
	return &SessionInterceptor{manager: manager, delegation: &applicationDelegation{
		headerValue: encoded, node: view.Audience.Node, delegatee: view.Delegatee,
	}}, nil
}

func invalidApplicationDelegation() error {
	return errors.New("Application Delegation is invalid")
}

func invalidApplicationDelegationPresentation() error {
	return connect.NewError(connect.CodeUnauthenticated, errors.New("Application Delegation presentation is invalid"))
}

func (i *SessionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if request.Header().Get("Authorization") != "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("multiple Application authentication schemes are forbidden"))
		}
		if headerValueCount(request.Header(), applicationDelegationHeader) != 0 {
			return nil, invalidApplicationDelegationPresentation()
		}
		header, generation, key, err := i.manager.authorization(ctx)
		if err != nil {
			return nil, err
		}
		if i.delegation != nil {
			if key.node != i.delegation.node || key.principal != i.delegation.delegatee {
				return nil, invalidApplicationDelegationPresentation()
			}
			request.Header().Set(applicationDelegationHeader, i.delegation.headerValue)
		}
		request.Header().Set("Authorization", header)
		response, err := next(ctx, request)
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			return response, err
		}
		i.manager.evictGeneration(generation)
		header, _, key, authErr := i.manager.authorization(ctx)
		if authErr != nil {
			return nil, authErr
		}
		if i.delegation != nil {
			if key.node != i.delegation.node || key.principal != i.delegation.delegatee {
				return nil, invalidApplicationDelegationPresentation()
			}
			request.Header().Set(applicationDelegationHeader, i.delegation.headerValue)
		}
		request.Header().Set("Authorization", header)
		return next(ctx, request)
	}
}

func headerValueCount(header http.Header, name string) int {
	count := 0
	for key, values := range header {
		if strings.EqualFold(key, name) {
			count += len(values)
		}
	}
	return count
}

func (i *SessionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *SessionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
