package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxAdminRequestIDBytes = 128

type AdminCommand struct {
	RequestID string
	Attempt   Attempt
}

type GrantProposal struct {
	Subject   string
	Actions   []Action
	Scope     ResourceScope
	NotBefore time.Time
	NotAfter  time.Time
}

type IssueGrantRequest struct {
	Command  AdminCommand
	Proposal GrantProposal
}

func (IssueGrantRequest) String() string   { return "issue Access Grant request [redacted]" }
func (IssueGrantRequest) GoString() string { return "issue Access Grant request [redacted]" }
func (IssueGrantRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"protected":"[redacted]"}`), nil
}

func (s *Service) IssueAccessGrant(ctx context.Context, request IssueGrantRequest) (string, error) {
	audit := newAdministrationAudit(request.Command.Attempt)
	succeeded := false
	defer func() {
		if !succeeded {
			audit.recordDenied(s, "admin_issue_grant_denied", request.Command.Attempt)
		}
	}()
	if s.grantIssuer == nil || validateAdminCommand(request.Command, "identity.grant.issue", "grant-proposal") != nil {
		return "", ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	issuerPublic := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	node, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil || node.String() != request.Command.Attempt.Binding.Audience.Node {
		return "", ErrUnavailable
	}
	payload, err := grantProposalPayload(node.String(), request.Command.Attempt.Binding.Audience, request.Proposal)
	if err != nil {
		return "", ErrInvalidArgument
	}
	proposalID, err := grantProposalID(payload)
	if err != nil || request.Command.Attempt.Resource.ID != proposalID {
		return "", ErrInvalidArgument
	}
	grant, err := s.grantIssuer.IssueAccessGrant(proto.Clone(payload).(*identityprotocol.AccessGrantPayload))
	if err != nil || grant == nil || !proto.Equal(grant.AccessGrantPayload(), payload) {
		return "", ErrUnavailable
	}
	id, index, sum, record, err := prepareGrantRecord(grant, issuerPublic, now)
	if err != nil {
		return "", ErrUnavailable
	}
	digest, err := adminDigest("issue-grant", payload)
	if err != nil {
		return "", ErrInternal
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	result := ""
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		call, admitErr := audit.admit(s, tx, transactionNow, request.Command.Attempt)
		if admitErr != nil {
			return admitErr
		}
		key := adminCommandKey(node.String(), call.Actor(), string(call.Action()), request.Command.RequestID)
		prior, found, commandErr := loadAdminCommand(tx, key, digest, identitycontract.AccessGrantPrefix)
		if commandErr != nil {
			return commandErr
		}
		if found {
			result = prior
			return audit.commitSuccessfulMutation(tx, "access_grant_issued")
		}
		if validateGrant(grant.AccessGrantPayload(), transactionNow) != nil {
			return ErrInvalidArgument
		}
		enrollmentKey, keyErr := enrollmentKey(node.String(), request.Proposal.Subject)
		if keyErr != nil {
			return keyErr
		}
		enrollmentRaw, enrolled, keyErr := tx.Get(enrollmentsBucket, enrollmentKey)
		if keyErr != nil {
			return keyErr
		}
		if !enrolled {
			return ErrInvalidArgument
		}
		if _, keyErr = decodeEnrollment(node.String(), request.Proposal.Subject, enrollmentRaw); keyErr != nil {
			return keyErr
		}
		if err := recordGrant(tx, id, index, sum, record); err != nil {
			return err
		}
		result = id
		if err := recordAdminCommand(tx, key, digest, result, identitycontract.AccessGrantPrefix); err != nil {
			return err
		}
		return audit.commitSuccessfulMutation(tx, "access_grant_issued")
	})
	if err != nil {
		return "", mapAdminError(err)
	}
	succeeded = true
	if err := s.flushAuditOutbox(ctx); err != nil {
		return "", ErrUnavailable
	}
	return result, nil
}

func grantProposalID(payload *identityprotocol.AccessGrantPayload) (string, error) {
	if payload == nil || payload.Audience == nil || payload.Scope == nil {
		return "", errInvalid
	}
	actions := make([][]byte, len(payload.Actions))
	for index := range payload.Actions {
		actions[index] = []byte(payload.Actions[index])
	}
	actionTuple := tuple(actions...)
	var scopeTuple []byte
	switch scope := payload.Scope.Scope.(type) {
	case *identityprotocol.ResourceScope_Node:
		scopeTuple = tuple([]byte{byte(ScopeNode)})
	case *identityprotocol.ResourceScope_PrincipalOwned:
		scopeTuple = tuple([]byte{byte(ScopePrincipalOwned)}, []byte(scope.PrincipalOwned.Owner))
	case *identityprotocol.ResourceScope_Exact:
		resource := scope.Exact.GetResource()
		if resource == nil {
			return "", errInvalid
		}
		scopeTuple = tuple([]byte{byte(ScopeExact)}, []byte(resource.Node), []byte(resource.Owner), []byte(resource.Kind), []byte(resource.CanonicalId))
	default:
		return "", errInvalid
	}
	var notBefore, notAfter [8]byte
	binary.BigEndian.PutUint64(notBefore[:], uint64(payload.NotBefore.AsTime().Unix()))
	binary.BigEndian.PutUint64(notAfter[:], uint64(payload.NotAfter.AsTime().Unix()))
	canonical := tuple([]byte(payload.Subject), []byte(payload.Audience.Node), []byte{byte(payload.Audience.Interface)}, uint32Bytes(payload.Audience.ProtocolMajor), actionTuple, scopeTuple, notBefore[:], notAfter[:])
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents:grant-proposal:v1\x00"))
	_, _ = hash.Write(canonical)
	return lowerASCII(sessionIDEncoding.EncodeToString(hash.Sum(nil))), nil
}

// GrantProposalResourceID returns the canonical exact resource selected by an
// IssueAccessGrant request. The server derives it; clients cannot choose it.
func GrantProposalResourceID(node string, audience Audience, proposal GrantProposal) (string, error) {
	payload, err := grantProposalPayload(node, audience, proposal)
	if err != nil {
		return "", ErrInvalidArgument
	}
	return grantProposalID(payload)
}

// ParseResourceScope converts the generated DTO into the access domain model.
func ParseResourceScope(scope *identityprotocol.ResourceScope, node string) (ResourceScope, error) {
	result, err := scopeFromPayload(scope, node)
	if err != nil {
		return ResourceScope{}, ErrInvalidArgument
	}
	return result, nil
}

// ResourceScopeFields returns an isolated wire DTO for list responses.
func ResourceScopeFields(scope ResourceScope, audience Audience) (*identityprotocol.ResourceScope, error) {
	result, err := scopeToProtocol(scope, audience)
	if err != nil {
		return nil, ErrInvalidArgument
	}
	return result, nil
}

func DeviceResourceID(subject, deviceID string) (string, error) {
	if _, err := identityprincipal.Parse(subject); err != nil {
		return "", ErrInvalidArgument
	}
	if _, err := identityprincipal.ParseDeviceID(deviceID); err != nil {
		return "", ErrInvalidArgument
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents:device-resource:v1\x00"))
	_, _ = hash.Write(tuple([]byte(subject), []byte(deviceID)))
	return lowerASCII(sessionIDEncoding.EncodeToString(hash.Sum(nil))), nil
}

// GrantAudienceForActions derives the single protected interface on which all
// proposed actions are registered. The Operator command remains the authority
// for issuance; this audience identifies the grant being issued.
func GrantAudienceForActions(node string, actions []Action) (Audience, error) {
	if _, err := identityprincipal.Parse(node); err != nil || len(actions) == 0 || len(actions) > identitycontract.MaxActions {
		return Audience{}, ErrInvalidArgument
	}
	var surface identityprotocol.Interface
	for index, action := range actions {
		if index > 0 && actions[index-1] >= action {
			return Audience{}, ErrInvalidArgument
		}
		actionSurface := identityprotocol.Interface_INTERFACE_UNSPECIFIED
		for _, candidate := range []identityprotocol.Interface{
			identityprotocol.Interface_INTERFACE_OPERATOR,
			identityprotocol.Interface_INTERFACE_APPLICATION,
		} {
			if parsed, err := ParseAction(candidate, string(action)); err == nil && parsed == action {
				if actionSurface != identityprotocol.Interface_INTERFACE_UNSPECIFIED {
					return Audience{}, ErrInvalidArgument
				}
				actionSurface = candidate
			}
		}
		if actionSurface == identityprotocol.Interface_INTERFACE_UNSPECIFIED ||
			surface != identityprotocol.Interface_INTERFACE_UNSPECIFIED && surface != actionSurface {
			return Audience{}, ErrInvalidArgument
		}
		surface = actionSurface
	}
	return Audience{Node: node, Interface: surface, ProtocolMajor: identitycontract.ProtocolMajor}, nil
}

func grantProposalPayload(node string, administrativeAudience Audience, proposal GrantProposal) (*identityprotocol.AccessGrantPayload, error) {
	if _, err := identityprincipal.Parse(proposal.Subject); err != nil ||
		administrativeAudience.Node != node ||
		administrativeAudience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR ||
		administrativeAudience.ProtocolMajor != identitycontract.ProtocolMajor ||
		proposal.Scope.Kind == ScopePrincipalOwned {
		return nil, errInvalid
	}
	grantAudience, err := GrantAudienceForActions(node, proposal.Actions)
	if err != nil {
		return nil, errInvalid
	}
	actions := make([]string, len(proposal.Actions))
	for index, action := range proposal.Actions {
		parsed, err := ParseAction(grantAudience.Interface, string(action))
		if err != nil {
			return nil, errInvalid
		}
		actions[index] = string(parsed)
	}
	scope, err := scopeToProtocol(proposal.Scope, grantAudience)
	if err != nil {
		return nil, errInvalid
	}
	for _, action := range proposal.Actions {
		if !registeredActionAllowsScope(grantAudience.Interface, action, proposal.Scope.Kind) {
			return nil, errInvalid
		}
	}
	return &identityprotocol.AccessGrantPayload{Version: identitycontract.Version, Issuer: node, Subject: proposal.Subject, Audience: protocolAudience(grantAudience), Actions: actions, Scope: scope, NotBefore: timestamppb.New(proposal.NotBefore), NotAfter: timestamppb.New(proposal.NotAfter)}, nil
}

func scopeToProtocol(scope ResourceScope, audience Audience) (*identityprotocol.ResourceScope, error) {
	switch scope.Kind {
	case ScopeNode:
		if scope.Exact.Node != audience.Node {
			return nil, errInvalid
		}
		return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}, nil
	case ScopePrincipalOwned:
		if scope.Owner.IsNone() {
			return nil, errInvalid
		}
		return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: scope.Owner.String()}}}, nil
	case ScopeExact:
		r, err := NewResourceRef(
			scope.Exact.Node,
			scope.Exact.Owner,
			string(scope.Exact.Kind),
			scope.Exact.ID,
		)
		if err != nil || r != scope.Exact || r.Node != audience.Node {
			return nil, errInvalid
		}
		return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Exact{Exact: &identityprotocol.ExactScope{Resource: &identityprotocol.ResourceRef{Node: r.Node, Owner: r.Owner.String(), Kind: string(r.Kind), CanonicalId: r.ID}}}}, nil
	default:
		return nil, errInvalid
	}
}

func validateAdminCommand(command AdminCommand, action, kind string) error {
	if len(command.RequestID) == 0 || len(command.RequestID) > maxAdminRequestIDBytes || string(command.Attempt.Action) != action || string(command.Attempt.Resource.Kind) != kind {
		return errInvalid
	}
	for _, value := range []byte(command.RequestID) {
		if value < 0x21 || value > 0x7e {
			return errInvalid
		}
	}
	return nil
}

func adminDigest(domain string, message proto.Message) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return digest, err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents:admin:" + domain + ":v1\x00"))
	_, _ = hash.Write(raw)
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func adminCommandKey(node, actor, action, requestID string) []byte {
	return tuple([]byte(node), []byte(actor), []byte(action), []byte(requestID))
}

func loadAdminCommand(tx storage.ReadTransaction, key []byte, digest [sha256.Size]byte, resultPrefix string) (string, bool, error) {
	record, found, err := tx.Get(adminCommandsBucket, key)
	if err != nil || !found {
		return "", false, err
	}
	if len(record) <= 2+sha256.Size+sha256.Size || record[0] != 1 {
		return "", false, fmt.Errorf("admin command record is corrupt")
	}
	if !bytes.Equal(record[1:1+sha256.Size], digest[:]) {
		return "", false, ErrConflict
	}
	prefixLength := int(record[1+sha256.Size])
	resultStart := 2 + sha256.Size
	resultEnd := len(record) - sha256.Size
	if prefixLength != len(resultPrefix) || resultEnd <= resultStart || resultStart+prefixLength > resultEnd || !bytes.Equal(record[resultStart:resultStart+prefixLength], []byte(resultPrefix)) {
		return "", false, fmt.Errorf("admin command record is corrupt")
	}
	expected := sha256.Sum256(append([]byte("ardents:admin-command-record:v1\x00"), record[:resultEnd]...))
	if !bytes.Equal(expected[:], record[resultEnd:]) {
		return "", false, fmt.Errorf("admin command record is corrupt")
	}
	return string(record[resultStart:resultEnd]), true, nil
}

func recordAdminCommand(tx storage.WriteTransaction, key []byte, digest [sha256.Size]byte, result, resultPrefix string) error {
	if result == "" || len(result) > identitycontract.MaxCanonicalResourceIDBytes || len(resultPrefix) > 255 || !bytes.HasPrefix([]byte(result), []byte(resultPrefix)) {
		return errInvalid
	}
	record := []byte{1}
	record = append(record, digest[:]...)
	record = append(record, byte(len(resultPrefix)))
	record = append(record, []byte(result)...)
	checksum := sha256.Sum256(append([]byte("ardents:admin-command-record:v1\x00"), record...))
	record = append(record, checksum[:]...)
	return tx.Put(adminCommandsBucket, key, record)
}

func mapAdminError(err error) error {
	if err == nil || err == ErrConflict || err == ErrPermissionDenied || err == ErrUnauthenticated || err == ErrInvalidArgument {
		return err
	}
	return ErrUnavailable
}
