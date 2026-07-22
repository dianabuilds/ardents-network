package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	applicationEnrollmentTicketRecordVersion byte = 1
	applicationEnrollmentTicketActive        byte = 1
	applicationEnrollmentTicketConsumed      byte = 2
)

var (
	applicationEnrollmentTicketDomain       = []byte("ardents:application-enrollment-ticket:v1\x00")
	applicationEnrollmentTicketRecordDomain = []byte("ardents:application-enrollment-ticket-record:v1\x00")
)

type ApplicationEnrollmentTicket [identitycontract.ApplicationEnrollmentTicketBytes]byte

func (ApplicationEnrollmentTicket) String() string { return "[redacted Application enrollment ticket]" }
func (ApplicationEnrollmentTicket) GoString() string {
	return "[redacted Application enrollment ticket]"
}
func (ApplicationEnrollmentTicket) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("[redacted Application enrollment ticket]"))
}
func (ApplicationEnrollmentTicket) MarshalJSON() ([]byte, error) {
	return []byte(`"[redacted Application enrollment ticket]"`), nil
}

type IssueApplicationEnrollmentTicketRequest struct {
	Attempt   Attempt
	Principal string
	Actions   []Action
}

type ApplicationEnrollmentTicketResult struct {
	Ticket    ApplicationEnrollmentTicket
	ExpiresAt time.Time
}

type EnrollApplicationRequest struct {
	Ticket        ApplicationEnrollmentTicket
	Challenge     Challenge
	Proof         EnrollmentProof
	RootPublicKey [ed25519.PublicKeySize]byte
	Credential    []byte
}

func (EnrollApplicationRequest) String() string   { return "Application enrollment request [redacted]" }
func (EnrollApplicationRequest) GoString() string { return "Application enrollment request [redacted]" }
func (EnrollApplicationRequest) MarshalJSON() ([]byte, error) {
	return []byte(`{"protected":"[redacted]"}`), nil
}

type EnrollApplicationResult struct {
	Principal      string
	CredentialID   string
	GrantID        string
	GrantExpiresAt time.Time
}

type applicationEnrollmentTicketRecord struct {
	state     byte
	issuedAt  time.Time
	expiresAt time.Time
	principal string
	actions   []Action
	digest    [sha256.Size]byte
}

func (s *Service) IssueApplicationEnrollmentTicket(ctx context.Context, request IssueApplicationEnrollmentTicketRequest) (ApplicationEnrollmentTicketResult, error) {
	if !s.applicationEnrollmentEnabled {
		return ApplicationEnrollmentTicketResult{}, ErrFeatureDisabled
	}
	if s.grantIssuer == nil || validateApplicationTicketIssueRequest(request) != nil {
		return ApplicationEnrollmentTicketResult{}, ErrInvalidArgument
	}
	now := canonicalNow(s.clock.Now())
	var ticket ApplicationEnrollmentTicket
	if s.random(ticket[:]) != nil || ticket == (ApplicationEnrollmentTicket{}) {
		return ApplicationEnrollmentTicketResult{}, ErrInternal
	}
	record := applicationEnrollmentTicketRecord{
		state: applicationEnrollmentTicketActive, issuedAt: now,
		expiresAt: now.Add(identitycontract.ApplicationEnrollmentTicketLifetime), principal: request.Principal,
		actions: append([]Action(nil), request.Actions...),
		digest:  applicationEnrollmentTicketDigest(ticket),
	}
	raw, err := encodeApplicationEnrollmentTicketRecord(record)
	if err != nil {
		return ApplicationEnrollmentTicketResult{}, ErrInvalidArgument
	}
	key := applicationEnrollmentTicketKey(request.Attempt.Binding.Audience.Node, request.Principal)
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	var actor, device string
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		if transactionNow.Before(record.issuedAt) || !transactionNow.Before(record.expiresAt) {
			return ErrUnauthenticated
		}
		call, session, admitErr := s.admitInTransaction(tx, transactionNow, request.Attempt)
		if admitErr != nil {
			return admitErr
		}
		actor, device = call.Actor(), session.DeviceID
		if call.Resource().ID != request.Principal || call.Resource().Kind != "principal" {
			return ErrInvalidArgument
		}
		existing, found, readErr := tx.Get(applicationEnrollmentTicketsBucket, key)
		if readErr != nil {
			return readErr
		}
		if found {
			prior, decodeErr := decodeApplicationEnrollmentTicketRecord(existing)
			if decodeErr != nil {
				return decodeErr
			}
			if prior.state == applicationEnrollmentTicketConsumed || transactionNow.Before(prior.expiresAt) {
				return ErrConflict
			}
		}
		return tx.Put(applicationEnrollmentTicketsBucket, key, raw)
	})
	if err != nil {
		s.record("denied", "application_enrollment_ticket_issue_denied", actor, device, request.Attempt.Binding.Audience)
		return ApplicationEnrollmentTicketResult{}, mapAdminError(err)
	}
	s.record("accepted", "application_enrollment_ticket_issued", actor, device, request.Attempt.Binding.Audience)
	return ApplicationEnrollmentTicketResult{Ticket: ticket, ExpiresAt: record.expiresAt}, nil
}

func validateApplicationTicketIssueRequest(request IssueApplicationEnrollmentTicketRequest) error {
	if request.Attempt.Binding.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR ||
		request.Attempt.Action != "identity.principal.enroll" || request.Attempt.Resource.Kind != "principal" ||
		request.Attempt.Resource.ID != request.Principal {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(request.Principal); err != nil || len(request.Actions) == 0 || len(request.Actions) > 2 {
		return errInvalid
	}
	for index, action := range request.Actions {
		if _, err := ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, string(action)); err != nil || index > 0 && request.Actions[index-1] >= action {
			return errInvalid
		}
	}
	return nil
}

func (s *Service) EnrollApplication(ctx context.Context, currentBinding AuthenticationBinding, request EnrollApplicationRequest) (EnrollApplicationResult, error) {
	if !s.applicationEnrollmentEnabled {
		return EnrollApplicationResult{}, ErrFeatureDisabled
	}
	now := canonicalNow(s.clock.Now())
	if s.grantIssuer == nil || currentBinding != request.Challenge.Binding || validateAuthenticationBinding(currentBinding) != nil ||
		currentBinding.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION ||
		request.Challenge.Purpose != identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF {
		return EnrollApplicationResult{}, ErrInvalidArgument
	}
	principal, err := identityprincipal.FromEd25519PublicKey(request.RootPublicKey[:])
	if err != nil || principal.String() != request.Challenge.Principal {
		return EnrollApplicationResult{}, ErrInvalidArgument
	}
	credential, err := ParseAndVerifyKeyCredential(request.Credential, now)
	if err != nil {
		return EnrollApplicationResult{}, ErrInvalidArgument
	}
	credentialPayload := credential.KeyCredentialPayload()
	if credentialPayload.Subject != principal.String() || !bytes.Equal(credentialPayload.RootPublicKey, request.RootPublicKey[:]) {
		return EnrollApplicationResult{}, ErrInvalidArgument
	}
	if !s.consumeEnrollmentProof(request.Proof, request.Challenge) {
		return EnrollApplicationResult{}, ErrUnauthenticated
	}
	issuerPublic := append(ed25519.PublicKey(nil), s.grantIssuer.PublicKey()...)
	node, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil || node.String() != currentBinding.Audience.Node {
		return EnrollApplicationResult{}, ErrUnavailable
	}
	var result EnrollApplicationResult
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
		transactionNow := canonicalNow(s.clock.Now())
		ticketRecord, consumeErr := consumeApplicationEnrollmentTicket(tx, node.String(), principal.String(), request.Ticket, currentBinding, transactionNow)
		if consumeErr != nil {
			return consumeErr
		}
		payload := &identityprotocol.AccessGrantPayload{
			Version: identitycontract.Version, Issuer: node.String(), Subject: principal.String(), Audience: protocolAudience(currentBinding.Audience),
			Actions:   actionStrings(ticketRecord.actions),
			Scope:     &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: principal.String()}}},
			NotBefore: timestamppb.New(transactionNow), NotAfter: timestamppb.New(transactionNow.Add(identitycontract.DefaultGrantLifetime)),
		}
		grant, issueErr := s.grantIssuer.IssueAccessGrant(proto.Clone(payload).(*identityprotocol.AccessGrantPayload))
		if issueErr != nil || grant == nil || !proto.Equal(grant.AccessGrantPayload(), payload) {
			return ErrUnavailable
		}
		grantID, grantIndex, grantHash, grantRaw, prepareErr := prepareGrantRecord(grant, issuerPublic, transactionNow)
		if prepareErr != nil {
			return prepareErr
		}
		enrollment := EnrollmentRecord{Node: node.String(), Principal: principal.String(), RootPublicKey: request.RootPublicKey, EnrolledAt: transactionNow}
		enrollmentKey, enrollmentRaw, encodeErr := encodeEnrollment(enrollment)
		if encodeErr != nil {
			return encodeErr
		}
		credentialKey, credentialRaw, credentialErr := prepareEnrollmentCredential(node.String(), principal.String(), credential, transactionNow)
		if credentialErr != nil {
			return credentialErr
		}
		if _, enrolled, readErr := tx.Get(enrollmentsBucket, enrollmentKey); readErr != nil {
			return readErr
		} else if enrolled {
			return ErrConflict
		}
		if writeErr := recordEnrollment(tx, enrollmentKey, enrollmentRaw); writeErr != nil {
			return writeErr
		}
		if writeErr := recordEnrollmentCredential(tx, credentialKey, credentialRaw); writeErr != nil {
			return writeErr
		}
		if writeErr := recordGrant(tx, grantID, grantIndex, grantHash, grantRaw); writeErr != nil {
			return writeErr
		}
		result = EnrollApplicationResult{Principal: principal.String(), CredentialID: credential.ID(), GrantID: grant.ID(), GrantExpiresAt: payload.NotAfter.AsTime()}
		return nil
	})
	if err != nil {
		s.record("denied", "application_enrollment_denied", "", "", currentBinding.Audience)
		if err == ErrUnauthenticated || err == ErrConflict || err == ErrInvalidArgument {
			return EnrollApplicationResult{}, err
		}
		return EnrollApplicationResult{}, ErrUnavailable
	}
	s.record("accepted", "application_enrolled", principal.String(), credentialPayload.DeviceId, currentBinding.Audience)
	return result, nil
}

func actionStrings(actions []Action) []string {
	result := make([]string, len(actions))
	for index := range actions {
		result[index] = string(actions[index])
	}
	return result
}

func applicationEnrollmentTicketKey(node, principal string) []byte {
	return tuple([]byte(node), []byte(principal))
}

func applicationEnrollmentTicketDigest(ticket ApplicationEnrollmentTicket) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write(applicationEnrollmentTicketDomain)
	_, _ = hash.Write(ticket[:])
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func encodeApplicationEnrollmentTicketRecord(record applicationEnrollmentTicketRecord) ([]byte, error) {
	if record.state != applicationEnrollmentTicketActive && record.state != applicationEnrollmentTicketConsumed ||
		record.issuedAt.Nanosecond() != 0 || !record.expiresAt.Equal(record.issuedAt.Add(identitycontract.ApplicationEnrollmentTicketLifetime)) ||
		len(record.principal) > 255 || validateApplicationTicketActions(record.actions) != nil {
		return nil, errInvalid
	}
	if _, err := identityprincipal.Parse(record.principal); err != nil {
		return nil, errInvalid
	}
	raw := []byte{applicationEnrollmentTicketRecordVersion, record.state, byte(len(record.principal)), byte(len(record.actions))}
	var times [16]byte
	binary.BigEndian.PutUint64(times[:8], uint64(record.issuedAt.Unix()))
	binary.BigEndian.PutUint64(times[8:], uint64(record.expiresAt.Unix()))
	raw = append(raw, times[:]...)
	raw = append(raw, record.digest[:]...)
	raw = append(raw, []byte(record.principal)...)
	for _, action := range record.actions {
		raw = append(raw, byte(len(action)))
		raw = append(raw, []byte(action)...)
	}
	checksum := sha256.Sum256(append(append([]byte(nil), applicationEnrollmentTicketRecordDomain...), raw...))
	return append(raw, checksum[:]...), nil
}

func decodeApplicationEnrollmentTicketRecord(raw []byte) (applicationEnrollmentTicketRecord, error) {
	var result applicationEnrollmentTicketRecord
	const fixed = 4 + 16 + sha256.Size + sha256.Size
	if len(raw) < fixed || raw[0] != applicationEnrollmentTicketRecordVersion || raw[1] != applicationEnrollmentTicketActive && raw[1] != applicationEnrollmentTicketConsumed {
		return result, fmt.Errorf("Application enrollment ticket record is corrupt")
	}
	payload, checksum := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	expected := sha256.Sum256(append(append([]byte(nil), applicationEnrollmentTicketRecordDomain...), payload...))
	if subtle.ConstantTimeCompare(checksum, expected[:]) != 1 {
		return result, fmt.Errorf("Application enrollment ticket record is corrupt")
	}
	principalLength, actionCount := int(raw[2]), int(raw[3])
	offset := 4
	issued := time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	expires := time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	copy(result.digest[:], raw[offset:offset+sha256.Size])
	offset += sha256.Size
	if offset+principalLength > len(payload) {
		return result, fmt.Errorf("Application enrollment ticket record is corrupt")
	}
	principal := string(raw[offset : offset+principalLength])
	offset += principalLength
	actions := make([]Action, actionCount)
	for index := range actions {
		if offset >= len(payload) {
			return result, fmt.Errorf("Application enrollment ticket record is corrupt")
		}
		length := int(raw[offset])
		offset++
		if length == 0 || offset+length > len(payload) {
			return result, fmt.Errorf("Application enrollment ticket record is corrupt")
		}
		actions[index] = Action(string(raw[offset : offset+length]))
		offset += length
	}
	if offset != len(payload) {
		return result, fmt.Errorf("Application enrollment ticket record is corrupt")
	}
	result.state, result.issuedAt, result.expiresAt, result.principal, result.actions = raw[1], issued, expires, principal, actions
	if _, err := encodeApplicationEnrollmentTicketRecord(result); err != nil {
		return applicationEnrollmentTicketRecord{}, fmt.Errorf("Application enrollment ticket record is corrupt")
	}
	return result, nil
}

func validateApplicationTicketActions(actions []Action) error {
	if len(actions) == 0 || len(actions) > 2 || !sort.SliceIsSorted(actions, func(i, j int) bool { return actions[i] < actions[j] }) {
		return errInvalid
	}
	for index, action := range actions {
		if _, err := ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, string(action)); err != nil || index > 0 && actions[index-1] == action {
			return errInvalid
		}
	}
	return nil
}

func consumeApplicationEnrollmentTicket(tx storage.WriteTransaction, node, principal string, ticket ApplicationEnrollmentTicket, binding AuthenticationBinding, now time.Time) (applicationEnrollmentTicketRecord, error) {
	raw, found, err := tx.Get(applicationEnrollmentTicketsBucket, applicationEnrollmentTicketKey(node, principal))
	if err != nil {
		return applicationEnrollmentTicketRecord{}, err
	}
	if !found {
		return applicationEnrollmentTicketRecord{}, ErrUnauthenticated
	}
	record, err := decodeApplicationEnrollmentTicketRecord(raw)
	if err != nil {
		return applicationEnrollmentTicketRecord{}, err
	}
	actual := applicationEnrollmentTicketDigest(ticket)
	if record.state != applicationEnrollmentTicketActive || record.principal != principal || binding.Audience.Node != node || binding.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION || now.Before(record.issuedAt) || !now.Before(record.expiresAt) || subtle.ConstantTimeCompare(actual[:], record.digest[:]) != 1 {
		return applicationEnrollmentTicketRecord{}, ErrUnauthenticated
	}
	record.state = applicationEnrollmentTicketConsumed
	consumed, err := encodeApplicationEnrollmentTicketRecord(record)
	if err != nil {
		return applicationEnrollmentTicketRecord{}, err
	}
	if err := tx.Put(applicationEnrollmentTicketsBucket, applicationEnrollmentTicketKey(node, principal), consumed); err != nil {
		return applicationEnrollmentTicketRecord{}, err
	}
	return record, nil
}
