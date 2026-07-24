// Package call carries Application admission facts across the RPC adapter
// without importing either wire protocol or the identity/access implementation.
// It does not own admission decisions or transport handling.
package call

import (
	"context"

	identityaccess "ardents/internal/identity/access"
)

// channelKey is deliberately non-zero-sized: Go may give distinct allocations
// of zero-sized values the same address, which would collapse channel isolation.
type channelKey struct{ marker byte }
type contextKey struct{ channel *channelKey }

type Injector struct{ channel *channelKey }
type Extractor struct{ channel *channelKey }

type principalFacts struct {
	Actor, Effective         string
	Node                     string
	Interface                int32
	ProtocolMajor            uint32
	Action                   string
	ResourceNode             string
	ResourceOwner            identityaccess.ResourceOwner
	ResourceKind, ResourceID string
}

type Call struct {
	principal *principalFacts
}

// NewChannel returns a matched injector/extractor pair. Context values from a
// different channel cannot be replayed into a handler using this extractor.
func NewChannel() (Injector, Extractor) {
	key := &channelKey{}
	return Injector{channel: key}, Extractor{channel: key}
}

func (i Injector) Valid() bool  { return i.channel != nil }
func (e Extractor) Valid() bool { return e.channel != nil }

// WithAuthorizedCall accepts only the sealed result of identity/access Admit.
// Callers cannot manufacture Actor or Effective through a public field mapper.
func (i Injector) WithAuthorizedCall(ctx context.Context, admitted identityaccess.AuthorizedCall) context.Context {
	if ctx == nil || !i.Valid() || !admitted.IsAdmitted() {
		return ctx
	}
	audience := admitted.Audience()
	resource := admitted.Resource()
	facts := principalFacts{
		Actor: admitted.Actor(), Effective: admitted.Effective(), Node: audience.Node,
		Interface: int32(audience.Interface), ProtocolMajor: audience.ProtocolMajor,
		Action: string(admitted.Action()), ResourceNode: resource.Node, ResourceOwner: resource.Owner,
		ResourceKind: string(resource.Kind), ResourceID: resource.ID,
	}
	if !validPrincipal(facts) {
		return ctx
	}
	copy := facts
	return context.WithValue(ctx, contextKey{channel: i.channel}, Call{principal: &copy})
}

func (e Extractor) Extract(ctx context.Context) (Call, bool) {
	if ctx == nil || !e.Valid() {
		return Call{}, false
	}
	stored, ok := ctx.Value(contextKey{channel: e.channel}).(Call)
	if !ok || !stored.IsAdmitted() {
		return Call{}, false
	}
	return stored.clone(), true
}

func validPrincipal(f principalFacts) bool {
	return f.Actor != "" && f.Effective != "" && f.Node != "" && f.Action != "" &&
		f.ResourceNode == f.Node && f.ResourceKind != "" && f.ResourceOwner.String() == f.Effective
}

func (c Call) clone() Call {
	result := Call{}
	if c.principal != nil {
		copy := *c.principal
		result.principal = &copy
	}
	return result
}

func (c Call) IsAdmitted() bool {
	return c.principal != nil && validPrincipal(*c.principal)
}
func (c Call) IsPrincipal() bool { return c.IsAdmitted() && c.principal != nil }
func (c Call) Actor() string {
	if c.principal != nil {
		return c.principal.Actor
	}
	return ""
}
func (c Call) Effective() string {
	if c.principal != nil {
		return c.principal.Effective
	}
	return ""
}
func (c Call) Action() string {
	if c.principal != nil {
		return c.principal.Action
	}
	return ""
}
func (c Call) Node() string {
	if c.principal != nil {
		return c.principal.Node
	}
	return ""
}
func (c Call) Interface() int32 {
	if c.principal != nil {
		return c.principal.Interface
	}
	return 0
}
func (c Call) ProtocolMajor() uint32 {
	if c.principal != nil {
		return c.principal.ProtocolMajor
	}
	return 0
}
func (c Call) ResourceNode() string {
	if c.principal != nil {
		return c.principal.ResourceNode
	}
	return ""
}
func (c Call) ResourceOwner() identityaccess.ResourceOwner {
	if c.principal != nil {
		return c.principal.ResourceOwner
	}
	return identityaccess.ResourceOwner{}
}
func (c Call) ResourceKind() string {
	if c.principal != nil {
		return c.principal.ResourceKind
	}
	return ""
}
func (c Call) ResourceID() string {
	if c.principal != nil {
		return c.principal.ResourceID
	}
	return ""
}
