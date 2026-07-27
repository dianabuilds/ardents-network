// Package authority adapts the protected Operator protocol to the Realm
// Authority domain. It does not own authentication, product policy, or state.
package authority

import (
	"errors"

	domain "ardents/internal/authority"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("authority resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind string) (identityaccess.ResourceTarget, error) {
	switch procedure {
	case ardentsv1connect.AuthorityServiceCreateRealmAuthorityProcedure:
		request, ok := message.(*protocol.CreateRealmAuthorityRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: domain.PrimaryAuthorityInstance,
		}, nil
	case ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure:
		request, ok := message.(*protocol.InspectRealmAuthorityRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			!domain.ValidRealmID(request.GetRealmId()) {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: request.GetRealmId(),
		}, nil
	default:
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
}
