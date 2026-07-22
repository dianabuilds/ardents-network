package access

import (
	"context"
	"errors"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
)

var (
	ErrPermissionDenied      = errors.New("identity access denied")
	ErrDelegationUnsupported = errors.New("delegation is not enabled")
	ErrInvalidResourceTarget = errors.New("identity resource target is invalid")
)

type Action string
type ResourceKind string
type ScopeKind uint8

const (
	ScopeNode ScopeKind = iota + 1
	ScopePrincipalOwned
	ScopeExact
)

type ResourceRef struct {
	Node  string
	Owner string
	Kind  ResourceKind
	ID    string
}
type ResourceScope struct {
	Kind  ScopeKind
	Owner string
	Exact ResourceRef
}

type Attempt struct {
	SessionSecret SessionSecret
	Binding       AuthenticationBinding
	Action        Action
	Resource      ResourceRef
	Delegation    []byte
}

type ResourceTarget struct {
	Kind ResourceKind
	ID   string
}

type ResourceFinalizer func(ResourceTarget, Audience, string, string) (ResourceRef, error)

type TargetAttempt struct {
	SessionSecret SessionSecret
	Binding       AuthenticationBinding
	Action        Action
	Target        ResourceTarget
	ResolveTarget func() (ResourceTarget, error)
	Finalize      ResourceFinalizer
	Delegation    []byte
}

type admissionSeal struct{}

type AuthorizedCall struct {
	actor, effective string
	audience         Audience
	action           Action
	resource         ResourceRef
	sessionID        string
	seal             *admissionSeal
}

func protocolAudience(audience Audience) *identityprotocol.Audience {
	return &identityprotocol.Audience{Node: audience.Node, Interface: audience.Interface, ProtocolMajor: audience.ProtocolMajor}
}

func (c AuthorizedCall) Actor() string         { return c.actor }
func (c AuthorizedCall) Effective() string     { return c.effective }
func (c AuthorizedCall) Audience() Audience    { return c.audience }
func (c AuthorizedCall) Action() Action        { return c.action }
func (c AuthorizedCall) Resource() ResourceRef { return c.resource }
func (c AuthorizedCall) SessionID() string     { return c.sessionID }
func (c AuthorizedCall) IsAdmitted() bool {
	return c.seal != nil && c.actor != "" && c.effective != "" && c.sessionID != ""
}

func ParseAction(surface identityprotocol.Interface, value string) (Action, error) {
	var contractSurface identitycontract.Interface
	if surface == identityprotocol.Interface_INTERFACE_OPERATOR {
		contractSurface = identitycontract.InterfaceOperator
	} else if surface == identityprotocol.Interface_INTERFACE_APPLICATION {
		contractSurface = identitycontract.InterfaceApplication
	} else {
		return "", ErrInvalidArgument
	}
	if !identitycontract.IsRegisteredAction(contractSurface, value) {
		return "", ErrInvalidArgument
	}
	return Action(value), nil
}

func NewResourceRef(node, owner, kind, id string) (ResourceRef, error) {
	if _, err := identityprincipal.Parse(node); err != nil {
		return ResourceRef{}, ErrInvalidArgument
	}
	contract, known := identitycontract.LookupResourceKind(kind)
	if !known {
		return ResourceRef{}, ErrInvalidArgument
	}
	if len(id) > identitycontract.MaxCanonicalResourceIDBytes || (!contract.AllowEmptyID && id == "") || contract.OwnerRequired != (owner != "") {
		return ResourceRef{}, ErrInvalidArgument
	}
	if owner != "" {
		if _, err := identityprincipal.Parse(owner); err != nil {
			return ResourceRef{}, ErrInvalidArgument
		}
	}
	return ResourceRef{Node: node, Owner: owner, Kind: ResourceKind(kind), ID: id}, nil
}

func (s ResourceScope) Matches(resource ResourceRef, audience Audience) bool {
	switch s.Kind {
	case ScopeNode:
		return resource.Node == audience.Node && resource.Node == s.Exact.Node
	case ScopePrincipalOwned:
		return resource.Node == audience.Node && resource.Owner != "" && resource.Owner == s.Owner
	case ScopeExact:
		return resource == s.Exact
	default:
		return false
	}
}

func registeredActionAllowsScope(surface identityprotocol.Interface, _ Action, scope ScopeKind) bool {
	// The frozen Operator catalogue grants content operations only through
	// Node or exact resources. Principal-owned scopes are reserved for the
	// Application surface, where ownership is backed by product state.
	if surface == identityprotocol.Interface_INTERFACE_OPERATOR && scope == ScopePrincipalOwned {
		return false
	}
	return true
}

func scopeFromPayload(scope *identityprotocol.ResourceScope, node string) (ResourceScope, error) {
	if scope == nil {
		return ResourceScope{}, errInvalid
	}
	switch x := scope.Scope.(type) {
	case *identityprotocol.ResourceScope_Node:
		return ResourceScope{Kind: ScopeNode, Exact: ResourceRef{Node: node}}, nil
	case *identityprotocol.ResourceScope_PrincipalOwned:
		if x.PrincipalOwned == nil {
			return ResourceScope{}, errInvalid
		}
		return ResourceScope{Kind: ScopePrincipalOwned, Owner: x.PrincipalOwned.Owner}, nil
	case *identityprotocol.ResourceScope_Exact:
		r := x.Exact.GetResource()
		if r == nil {
			return ResourceScope{}, errInvalid
		}
		ref, err := NewResourceRef(r.Node, r.Owner, r.Kind, r.CanonicalId)
		if err != nil {
			return ResourceScope{}, errInvalid
		}
		return ResourceScope{Kind: ScopeExact, Exact: ref}, nil
	default:
		return ResourceScope{}, errInvalid
	}
}

func (s *Service) Admit(ctx context.Context, attempt Attempt) (AuthorizedCall, error) {
	return s.admit(ctx, attempt.Binding, func(tx storage.ReadTransaction, now time.Time) (AuthorizedCall, Session, error) {
		return s.admitInTransaction(tx, now, attempt)
	})
}

func (s *Service) AdmitTarget(ctx context.Context, attempt TargetAttempt) (AuthorizedCall, error) {
	if attempt.Finalize == nil {
		return AuthorizedCall{}, ErrInvalidArgument
	}
	return s.admit(ctx, attempt.Binding, func(tx storage.ReadTransaction, now time.Time) (AuthorizedCall, Session, error) {
		return s.admitResolvedInTransaction(tx, now, attempt.SessionSecret, attempt.Binding, attempt.Action, attempt.Delegation, func(actor, effective string) (ResourceRef, error) {
			target := attempt.Target
			if attempt.ResolveTarget != nil {
				var err error
				target, err = attempt.ResolveTarget()
				if err != nil {
					return ResourceRef{}, ErrInvalidResourceTarget
				}
			}
			return attempt.Finalize(target, attempt.Binding.Audience, actor, effective)
		})
	})
}

func (s *Service) admit(ctx context.Context, binding AuthenticationBinding, invoke func(storage.ReadTransaction, time.Time) (AuthorizedCall, Session, error)) (AuthorizedCall, error) {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	now := canonicalNow(s.clock.Now())
	var call AuthorizedCall
	var session Session
	err := s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
		var admitErr error
		call, session, admitErr = invoke(tx, now)
		return admitErr
	})
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			s.record("denied", "session_or_device_invalid", session.Principal, session.DeviceID, binding.Audience)
			return AuthorizedCall{}, ErrUnauthenticated
		}
		if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrInvalidResourceTarget) || errors.Is(err, ErrDelegationUnsupported) {
			s.record("denied", "access_denied", session.Principal, session.DeviceID, binding.Audience)
			return AuthorizedCall{}, err
		}
		s.record("denied", "store_unavailable", session.Principal, session.DeviceID, binding.Audience)
		return AuthorizedCall{}, ErrUnavailable
	}
	s.record("accepted", "access_admitted", session.Principal, session.DeviceID, binding.Audience)
	return call, nil
}

// admitInTransaction repeats all mutable authority checks in the supplied
// durable snapshot. The caller must hold deviceMu across this callback.
func (s *Service) admitInTransaction(tx storage.ReadTransaction, now time.Time, attempt Attempt) (AuthorizedCall, Session, error) {
	return s.admitResolvedInTransaction(tx, now, attempt.SessionSecret, attempt.Binding, attempt.Action, attempt.Delegation, func(string, string) (ResourceRef, error) {
		resource, err := NewResourceRef(attempt.Resource.Node, attempt.Resource.Owner, string(attempt.Resource.Kind), attempt.Resource.ID)
		if err != nil || resource.Node != attempt.Binding.Audience.Node {
			return ResourceRef{}, ErrInvalidArgument
		}
		return resource, nil
	})
}

func (s *Service) admitResolvedInTransaction(tx storage.ReadTransaction, now time.Time, secret SessionSecret, binding AuthenticationBinding, requestedAction Action, delegation []byte, finalize func(string, string) (ResourceRef, error)) (AuthorizedCall, Session, error) {
	if len(delegation) > 0 {
		return AuthorizedCall{}, Session{}, ErrDelegationUnsupported
	}
	action, err := ParseAction(binding.Audience.Interface, string(requestedAction))
	if err != nil {
		return AuthorizedCall{}, Session{}, ErrInvalidArgument
	}
	session, found := s.sessions.get(now, secret)
	if !found || session.Binding != binding {
		return AuthorizedCall{}, Session{}, ErrUnauthenticated
	}
	key, err := deviceRevocationKey(session.Binding.Audience.Node, session.Principal, session.DeviceID)
	if err != nil {
		return AuthorizedCall{}, session, err
	}
	revoked, err := deviceRevoked(tx, key)
	if err != nil {
		return AuthorizedCall{}, session, err
	}
	if revoked {
		s.sessions.invalidateDevice(session.DeviceID)
		return AuthorizedCall{}, session, ErrUnauthenticated
	}
	effective := session.Principal
	resource, err := finalize(session.Principal, effective)
	if errors.Is(err, ErrInvalidResourceTarget) {
		return AuthorizedCall{}, session, ErrInvalidResourceTarget
	}
	if err != nil || resource.Node != binding.Audience.Node {
		return AuthorizedCall{}, session, ErrInvalidArgument
	}
	resource, err = NewResourceRef(resource.Node, resource.Owner, string(resource.Kind), resource.ID)
	if err != nil {
		return AuthorizedCall{}, session, ErrInvalidArgument
	}
	prefix := grantIndexPrefix(binding.Audience, session.Principal)
	matched, err := grantMatches(tx, now, session.Principal, binding.Audience, action, resource, prefix)
	if err != nil {
		return AuthorizedCall{}, session, err
	}
	if !matched {
		return AuthorizedCall{}, session, ErrPermissionDenied
	}
	call := AuthorizedCall{
		actor:     session.Principal,
		effective: effective,
		audience:  binding.Audience,
		action:    action,
		resource:  resource,
		sessionID: session.ID,
		seal:      &admissionSeal{},
	}
	return call, session, nil
}

type EnrollmentRecord struct {
	Node, Principal string
	RootPublicKey   [32]byte
	EnrolledAt      time.Time
}
