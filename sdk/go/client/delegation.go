package client

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkidentity "ardents/sdk/go/identity"
	"ardents/sdk/go/internal/adapter"
)

// DelegationRequest contains every fact shown to the delegating Principal.
// The Application Audience and the no-redelegation rule are fixed by v1 and
// cannot be selected by the caller.
type DelegationRequest struct {
	DelegatorPrincipal   string
	ApplicationPrincipal string
	NodePrincipal        string
	Actions              []string
	Scope                sdkidentity.ResourceScope
	NotBefore            time.Time
	NotAfter             time.Time
}

// DelegationProposal is a validated, canonical consent request. Its fields are
// sealed so a signer cannot receive a proposal different from the displayed
// consent without constructing a new validated proposal.
type DelegationProposal struct {
	request DelegationRequest
	valid   bool
}

// DelegationSigner is a typed signing capability. It cannot sign arbitrary
// bytes and returns an immutable identity artifact rather than a bearer value.
type DelegationSigner interface {
	SignDelegation(context.Context, DelegationProposal) (*sdkidentity.Artifact, error)
}

// NewDelegationProposal validates and canonicalizes the exact v1 consent.
func NewDelegationProposal(input DelegationRequest) (DelegationProposal, error) {
	request := DelegationRequest{
		DelegatorPrincipal:   input.DelegatorPrincipal,
		ApplicationPrincipal: input.ApplicationPrincipal,
		NodePrincipal:        input.NodePrincipal,
		Actions:              append([]string(nil), input.Actions...), Scope: input.Scope,
		NotBefore: input.NotBefore.UTC(), NotAfter: input.NotAfter.UTC(),
	}
	if !validPrincipalID(request.DelegatorPrincipal) ||
		!validPrincipalID(request.ApplicationPrincipal) ||
		!validPrincipalID(request.NodePrincipal) ||
		request.DelegatorPrincipal == request.ApplicationPrincipal {
		return DelegationProposal{}, errors.New("Delegation consent has invalid Principal binding")
	}
	if !canonicalDelegationInterval(request.NotBefore, request.NotAfter) {
		return DelegationProposal{}, errors.New("Delegation consent has invalid validity interval")
	}
	if !identitycontract.ValidActionCount(len(request.Actions)) {
		return DelegationProposal{}, errors.New("Delegation consent has invalid actions")
	}
	sort.Strings(request.Actions)
	for index, action := range request.Actions {
		if !identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, action) ||
			(index > 0 && request.Actions[index-1] == action) {
			return DelegationProposal{}, errors.New("Delegation consent has invalid actions")
		}
	}
	if !validDelegationScope(request.Scope, request.NodePrincipal) {
		return DelegationProposal{}, errors.New("Delegation consent has invalid resource scope")
	}
	return DelegationProposal{request: request, valid: true}, nil
}

// ConsentText returns the deterministic human-readable consent. It is not the
// signed representation; the typed signer constructs the canonical artifact
// with the fixed Delegation signature domain.
func (p DelegationProposal) ConsentText() string {
	if !p.valid {
		return ""
	}
	var text strings.Builder
	text.WriteString("Ardents Delegation Consent v1\n")
	text.WriteString("Delegator Principal: " + p.request.DelegatorPrincipal + "\n")
	text.WriteString("Node Principal: " + p.request.NodePrincipal + "\n")
	text.WriteString("Application Principal: " + p.request.ApplicationPrincipal + "\n")
	text.WriteString("Actions:\n")
	for _, action := range p.request.Actions {
		text.WriteString("- " + action + "\n")
	}
	text.WriteString("Resource scope: " + delegationScopeText(p.request.Scope) + "\n")
	text.WriteString("Valid from: " + p.request.NotBefore.Format(time.RFC3339) + "\n")
	text.WriteString("Expires at: " + p.request.NotAfter.Format(time.RFC3339) + "\n")
	text.WriteString("Redelegation: forbidden (one hop only)")
	return text.String()
}

// Spec supplies the typed canonical fields to a custody implementation. The
// Credential must identify the device key used by that implementation.
func (p DelegationProposal) Spec(credential *sdkidentity.Artifact) sdkidentity.DelegationSpec {
	if !p.valid {
		return sdkidentity.DelegationSpec{}
	}
	return sdkidentity.DelegationSpec{
		Delegator: p.request.DelegatorPrincipal, Delegatee: p.request.ApplicationPrincipal,
		Audience: sdkidentity.Audience{Node: p.request.NodePrincipal, Interface: sdkidentity.InterfaceApplication, ProtocolMajor: identitycontract.ProtocolMajor},
		Actions:  append([]string(nil), p.request.Actions...), Scope: p.request.Scope,
		NotBefore: p.request.NotBefore, NotAfter: p.request.NotAfter, Credential: credential,
	}
}

// CreateDelegation invokes one typed signer and verifies that the returned
// immutable artifact exactly matches the consent shown to the Principal.
func CreateDelegation(ctx context.Context, proposal DelegationProposal, signer DelegationSigner) (*sdkidentity.Artifact, error) {
	if !proposal.valid || signer == nil {
		return nil, errors.New("Delegation signer is unavailable")
	}
	artifact, err := signer.SignDelegation(ctx, proposal)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, errors.New("Delegation signer is unavailable")
	}
	if !delegationMatchesProposal(artifact, proposal) {
		return nil, errors.New("signed Delegation does not match consent")
	}
	return artifact, nil
}

func delegationMatchesProposal(artifact *sdkidentity.Artifact, proposal DelegationProposal) bool {
	if artifact == nil || artifact.Kind() != sdkidentity.KindDelegation || !proposal.valid {
		return false
	}
	delegation := artifact.Delegation()
	request := proposal.request
	if delegation == nil || delegation.Delegator != request.DelegatorPrincipal ||
		delegation.Delegatee != request.ApplicationPrincipal ||
		delegation.Audience != (sdkidentity.Audience{Node: request.NodePrincipal, Interface: sdkidentity.InterfaceApplication, ProtocolMajor: identitycontract.ProtocolMajor}) ||
		delegation.Scope != request.Scope ||
		!delegation.NotBefore.Equal(request.NotBefore) || !delegation.NotAfter.Equal(request.NotAfter) ||
		len(delegation.Actions) != len(request.Actions) {
		return false
	}
	for index := range request.Actions {
		if delegation.Actions[index] != request.Actions[index] {
			return false
		}
	}
	return true
}

func canonicalDelegationInterval(start, end time.Time) bool {
	return start.Nanosecond() == 0 && end.Nanosecond() == 0 &&
		start.Unix() >= identitycontract.LowerTimestampUnix && start.Unix() < identitycontract.UpperTimestampUnix &&
		end.Unix() >= identitycontract.LowerTimestampUnix && end.Unix() < identitycontract.UpperTimestampUnix &&
		end.After(start) && end.Sub(start) <= identitycontract.MaxDelegationLifetime
}

func validDelegationScope(scope sdkidentity.ResourceScope, node string) bool {
	switch scope.Kind {
	case sdkidentity.ScopeNode:
		return scope.Owner == "" && scope.Resource == (sdkidentity.ResourceRef{})
	case sdkidentity.ScopePrincipalOwned:
		return validPrincipalID(scope.Owner) && scope.Resource == (sdkidentity.ResourceRef{})
	case sdkidentity.ScopeExact:
		resource := scope.Resource
		contract, known := identitycontract.LookupResourceKind(resource.Kind)
		ownerPresent := resource.Owner != ""
		return scope.Owner == "" && resource.Node == node && known &&
			ownerPresent == contract.OwnerRequired && (!ownerPresent || validPrincipalID(resource.Owner)) &&
			((contract.AllowEmptyID && resource.CanonicalID == "") || validConsentToken(resource.CanonicalID, identitycontract.MaxCanonicalResourceIDBytes))
	default:
		return false
	}
}

func validConsentToken(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func delegationScopeText(scope sdkidentity.ResourceScope) string {
	switch scope.Kind {
	case sdkidentity.ScopeNode:
		return "node"
	case sdkidentity.ScopePrincipalOwned:
		return "principal-owned(owner=" + scope.Owner + ")"
	case sdkidentity.ScopeExact:
		return fmt.Sprintf("exact(node=%q, owner=%q, kind=%q, id=%q)", scope.Resource.Node, scope.Resource.Owner, scope.Resource.Kind, scope.Resource.CanonicalID)
	default:
		return "invalid"
	}
}

func validPrincipalID(value string) bool {
	return value == strings.TrimSpace(value) && adapter.ValidPrincipalID(value)
}
