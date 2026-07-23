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
	Owner ResourceOwner
	Kind  ResourceKind
	ID    string
}
type ResourceScope struct {
	Kind  ScopeKind
	Owner ResourceOwner
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

type admissionTrace struct {
	Actor, Effective string
	Action           Action
	GrantIDs         []string
	DelegationID     string
}

func deniedAdmission(session Session, _ admissionTrace, cause error) (AuthorizedCall, Session, error) {
	return AuthorizedCall{}, session, cause
}

type AuthorizedCall struct {
	actor, effective string
	audience         Audience
	action           Action
	resource         ResourceRef
	sessionID        string
	grantIDs         []string
	delegationID     string
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
func (c AuthorizedCall) GrantIDs() []string    { return append([]string(nil), c.grantIDs...) }
func (c AuthorizedCall) DelegationID() string  { return c.delegationID }
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

func NewResourceRef(node string, owner ResourceOwner, kind, id string) (ResourceRef, error) {
	if _, err := identityprincipal.Parse(node); err != nil {
		return ResourceRef{}, ErrInvalidArgument
	}
	contract, known := identitycontract.LookupResourceKind(kind)
	if !known {
		return ResourceRef{}, ErrInvalidArgument
	}
	if len(id) > identitycontract.MaxCanonicalResourceIDBytes || (!contract.AllowEmptyID && id == "") || contract.OwnerRequired != !owner.IsNone() {
		return ResourceRef{}, ErrInvalidArgument
	}
	return ResourceRef{Node: node, Owner: owner, Kind: ResourceKind(kind), ID: id}, nil
}

func (s ResourceScope) Matches(resource ResourceRef, audience Audience) bool {
	switch s.Kind {
	case ScopeNode:
		return resource.Node == audience.Node && resource.Node == s.Exact.Node
	case ScopePrincipalOwned:
		return resource.Node == audience.Node && !resource.Owner.IsNone() && resource.Owner.Equal(s.Owner)
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
		owner, err := ParseResourceOwner(x.PrincipalOwned.Owner)
		if err != nil || owner.IsNone() {
			return ResourceScope{}, errInvalid
		}
		return ResourceScope{Kind: ScopePrincipalOwned, Owner: owner}, nil
	case *identityprotocol.ResourceScope_Exact:
		r := x.Exact.GetResource()
		if r == nil {
			return ResourceScope{}, errInvalid
		}
		owner, ownerErr := ParseResourceOwner(r.Owner)
		if ownerErr != nil {
			return ResourceScope{}, errInvalid
		}
		ref, err := NewResourceRef(r.Node, owner, r.Kind, r.CanonicalId)
		if err != nil {
			return ResourceScope{}, errInvalid
		}
		return ResourceScope{Kind: ScopeExact, Exact: ref}, nil
	default:
		return ResourceScope{}, errInvalid
	}
}

func (s *Service) Admit(ctx context.Context, attempt Attempt) (AuthorizedCall, error) {
	return s.admit(ctx, attempt.Binding, func(tx storage.ReadTransaction, now time.Time) (AuthorizedCall, Session, admissionTrace, error) {
		var trace admissionTrace
		call, session, err := s.admitInTransactionWithTrace(tx, now, attempt, &trace)
		return call, session, trace, err
	})
}

func (s *Service) AdmitTarget(ctx context.Context, attempt TargetAttempt) (AuthorizedCall, error) {
	if attempt.Finalize == nil {
		return AuthorizedCall{}, ErrInvalidArgument
	}
	return s.admit(ctx, attempt.Binding, func(tx storage.ReadTransaction, now time.Time) (AuthorizedCall, Session, admissionTrace, error) {
		var trace admissionTrace
		call, session, err := s.admitResolvedInTransaction(tx, now, attempt.SessionSecret, attempt.Binding, attempt.Action, attempt.Delegation, &trace, func(actor, effective string) (ResourceRef, error) {
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
		return call, session, trace, err
	})
}

func (s *Service) admit(ctx context.Context, binding AuthenticationBinding, invoke func(storage.ReadTransaction, time.Time) (AuthorizedCall, Session, admissionTrace, error)) (AuthorizedCall, error) {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	now := canonicalNow(s.clock.Now())
	var call AuthorizedCall
	var session Session
	var trace admissionTrace
	err := s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
		var admitErr error
		call, session, trace, admitErr = invoke(tx, now)
		return admitErr
	})
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			s.recordAdmission("denied", "session_or_device_invalid", session, binding.Audience, trace)
			return AuthorizedCall{}, ErrUnauthenticated
		}
		if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrInvalidResourceTarget) {
			s.recordAdmission("denied", "access_denied", session, binding.Audience, trace)
			return AuthorizedCall{}, err
		}
		s.recordAdmission("denied", "store_unavailable", session, binding.Audience, trace)
		return AuthorizedCall{}, ErrUnavailable
	}
	s.recordAdmission("accepted", "access_admitted", session, call.audience, admissionTrace{
		Actor: call.actor, Effective: call.effective, Action: call.action,
		GrantIDs: call.GrantIDs(), DelegationID: call.delegationID,
	})
	return call, nil
}

func (s *Service) recordAdmission(outcome, reason string, session Session, audience Audience, trace admissionTrace) {
	if s.audit == nil {
		return
	}
	s.audit.RecordIdentityAccess(AuditEvent{
		Outcome: outcome, Reason: reason, Principal: session.Principal, DeviceID: session.DeviceID,
		Audience: audience, Actor: trace.Actor, Effective: trace.Effective, Action: trace.Action,
		GrantIDs: append([]string(nil), trace.GrantIDs...), DelegationID: trace.DelegationID,
	})
}

// admitInTransaction repeats all mutable authority checks in the supplied
// durable snapshot. The caller must hold deviceMu across this callback.
func (s *Service) admitInTransaction(tx storage.ReadTransaction, now time.Time, attempt Attempt) (AuthorizedCall, Session, error) {
	return s.admitInTransactionWithTrace(tx, now, attempt, nil)
}

func (s *Service) admitInTransactionWithTrace(tx storage.ReadTransaction, now time.Time, attempt Attempt, trace *admissionTrace) (AuthorizedCall, Session, error) {
	return s.admitResolvedInTransaction(tx, now, attempt.SessionSecret, attempt.Binding, attempt.Action, attempt.Delegation, trace, func(string, string) (ResourceRef, error) {
		resource, err := NewResourceRef(attempt.Resource.Node, attempt.Resource.Owner, string(attempt.Resource.Kind), attempt.Resource.ID)
		if err != nil || resource.Node != attempt.Binding.Audience.Node {
			return ResourceRef{}, ErrInvalidArgument
		}
		return resource, nil
	})
}

func (s *Service) admitResolvedInTransaction(tx storage.ReadTransaction, now time.Time, secret SessionSecret, binding AuthenticationBinding, requestedAction Action, delegation []byte, traceOutput *admissionTrace, finalize func(string, string) (ResourceRef, error)) (AuthorizedCall, Session, error) {
	trace := admissionTrace{}
	defer func() {
		if traceOutput != nil {
			traceOutput.Actor = trace.Actor
			traceOutput.Effective = trace.Effective
			traceOutput.Action = trace.Action
			traceOutput.GrantIDs = append([]string(nil), trace.GrantIDs...)
			traceOutput.DelegationID = trace.DelegationID
		}
	}()
	// Reject an oversized presentation before protobuf parsing or hashing.
	if len(delegation) > maxArtifactBytes {
		return AuthorizedCall{}, Session{}, ErrUnauthenticated
	}
	action, err := ParseAction(binding.Audience.Interface, string(requestedAction))
	if err != nil {
		return AuthorizedCall{}, Session{}, ErrInvalidArgument
	}
	session, found := s.sessions.get(now, secret)
	if !found || session.Binding != binding {
		return AuthorizedCall{}, Session{}, ErrUnauthenticated
	}
	trace.Actor, trace.Action = session.Principal, action
	if len(delegation) == 0 {
		trace.Effective = session.Principal
	}
	key, err := deviceRevocationKey(session.Binding.Audience.Node, session.Principal, session.DeviceID)
	if err != nil {
		return deniedAdmission(session, trace, err)
	}
	revoked, err := deviceRevoked(tx, key)
	if err != nil {
		return deniedAdmission(session, trace, err)
	}
	if revoked {
		s.sessions.invalidateDevice(session.DeviceID)
		return deniedAdmission(session, trace, ErrUnauthenticated)
	}
	effective := session.Principal
	var delegationArtifact *Artifact
	if len(delegation) > 0 {
		delegationArtifact, err = ParseAndVerifyDelegation(delegation, now)
		if err != nil {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		payload := delegationArtifact.DelegationPayload()
		if payload == nil || payload.Delegatee != session.Principal || audienceFromProtocol(payload.Audience) != binding.Audience {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		trace.Effective = payload.Delegator
		trace.DelegationID = delegationArtifact.ID()
		credential := payload.Credential.GetPayload()
		delegatorRevocationKey, keyErr := deviceRevocationKey(binding.Audience.Node, payload.Delegator, credential.GetDeviceId())
		if keyErr != nil {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		delegatorDeviceRevoked, repositoryErr := deviceRevoked(tx, delegatorRevocationKey)
		if repositoryErr != nil {
			return deniedAdmission(session, trace, repositoryErr)
		}
		if delegatorDeviceRevoked {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		delegationIsRevoked, repositoryErr := delegationRevoked(tx, delegationArtifact)
		if repositoryErr != nil {
			return deniedAdmission(session, trace, repositoryErr)
		}
		if delegationIsRevoked {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		effective = payload.Delegator
	}
	resource, err := finalize(session.Principal, effective)
	if errors.Is(err, ErrInvalidResourceTarget) {
		return deniedAdmission(session, trace, ErrInvalidResourceTarget)
	}
	if err != nil || resource.Node != binding.Audience.Node {
		return deniedAdmission(session, trace, ErrInvalidArgument)
	}
	resource, err = NewResourceRef(resource.Node, resource.Owner, string(resource.Kind), resource.ID)
	if err != nil {
		return deniedAdmission(session, trace, ErrInvalidArgument)
	}
	actorPrefix := grantIndexPrefix(binding.Audience, session.Principal)
	actorGrantIDs, err := matchingGrantIDs(tx, now, session.Principal, binding.Audience, action, resource, actorPrefix)
	if err != nil {
		return deniedAdmission(session, trace, err)
	}
	trace.GrantIDs = append([]string(nil), actorGrantIDs...)
	if len(actorGrantIDs) == 0 {
		return deniedAdmission(session, trace, ErrPermissionDenied)
	}
	grantIDs := actorGrantIDs
	delegationID := ""
	if delegationArtifact != nil {
		payload := delegationArtifact.DelegationPayload()
		hasAction := false
		for _, delegatedAction := range payload.Actions {
			if delegatedAction == string(action) {
				hasAction = true
				break
			}
		}
		delegationScope, scopeErr := scopeFromPayload(payload.Scope, binding.Audience.Node)
		if scopeErr != nil {
			return deniedAdmission(session, trace, ErrUnauthenticated)
		}
		if !hasAction || !delegationScope.Matches(resource, binding.Audience) {
			return deniedAdmission(session, trace, ErrPermissionDenied)
		}
		effectivePrefix := grantIndexPrefix(binding.Audience, effective)
		effectiveGrantIDs, matchErr := matchingGrantIDs(tx, now, effective, binding.Audience, action, resource, effectivePrefix)
		if matchErr != nil {
			return deniedAdmission(session, trace, matchErr)
		}
		if len(effectiveGrantIDs) == 0 {
			return deniedAdmission(session, trace, ErrPermissionDenied)
		}
		grantIDs = append(append([]string(nil), actorGrantIDs...), effectiveGrantIDs...)
		delegationID = delegationArtifact.ID()
		trace.GrantIDs = append([]string(nil), grantIDs...)
	}
	call := AuthorizedCall{
		actor:        session.Principal,
		effective:    effective,
		audience:     binding.Audience,
		action:       action,
		resource:     resource,
		sessionID:    session.ID,
		grantIDs:     grantIDs,
		delegationID: delegationID,
		seal:         &admissionSeal{},
	}
	return call, session, nil
}

type EnrollmentRecord struct {
	Node, Principal string
	RootPublicKey   [32]byte
	EnrolledAt      time.Time
}
