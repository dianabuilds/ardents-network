package access

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
)

const (
	auditOutboxRecordVersion = 1
	maxAuditOutboxRecords    = 4096
	applicationEnrollAction  = Action("identity.application.enroll")
)

type persistedAuditEvent struct {
	Version       uint32   `json:"version"`
	Outcome       string   `json:"outcome"`
	Reason        string   `json:"reason"`
	Principal     string   `json:"principal,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	Node          string   `json:"node"`
	Interface     int32    `json:"interface"`
	ProtocolMajor uint32   `json:"protocol_major"`
	Actor         string   `json:"actor,omitempty"`
	Effective     string   `json:"effective,omitempty"`
	Action        string   `json:"action"`
	GrantIDs      []string `json:"grant_ids,omitempty"`
	DelegationID  string   `json:"delegation_id,omitempty"`
	CorrelationID string   `json:"correlation_id"`
}

func recordAuditOutbox(tx storage.WriteTransaction, event AuditEvent) error {
	raw, err := encodeAuditOutboxEvent(event)
	if err != nil {
		return err
	}
	count := 0
	if err := tx.ForEach(auditOutboxBucket, func(_, _ []byte) error {
		count++
		if count >= maxAuditOutboxRecords {
			return ErrResourceExhausted
		}
		return nil
	}); err != nil {
		return err
	}
	key := []byte(event.CorrelationID)
	if _, found, err := tx.Get(auditOutboxBucket, key); err != nil {
		return err
	} else if found {
		return ErrConflict
	}
	return tx.Put(auditOutboxBucket, key, raw)
}

func encodeAuditOutboxEvent(event AuditEvent) ([]byte, error) {
	if err := validateDurableAuditEvent(event); err != nil {
		return nil, err
	}
	record := persistedAuditEvent{
		Version: auditOutboxRecordVersion, Outcome: event.Outcome, Reason: event.Reason,
		Principal: event.Principal, DeviceID: event.DeviceID,
		Node: event.Audience.Node, Interface: int32(event.Audience.Interface), ProtocolMajor: event.Audience.ProtocolMajor,
		Actor: event.Actor, Effective: event.Effective, Action: string(event.Action),
		GrantIDs: append([]string(nil), event.GrantIDs...), DelegationID: event.DelegationID,
		CorrelationID: event.CorrelationID,
	}
	return json.Marshal(record)
}

func decodeAuditOutboxEvent(key, raw []byte) (AuditEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record persistedAuditEvent
	if err := decoder.Decode(&record); err != nil {
		return AuditEvent{}, errInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return AuditEvent{}, errInvalid
	}
	if record.Version != auditOutboxRecordVersion || record.CorrelationID != string(key) {
		return AuditEvent{}, errInvalid
	}
	event := AuditEvent{
		Outcome: record.Outcome, Reason: record.Reason, Principal: record.Principal, DeviceID: record.DeviceID,
		Audience: Audience{Node: record.Node, Interface: identityprotocol.Interface(record.Interface), ProtocolMajor: record.ProtocolMajor},
		Actor:    record.Actor, Effective: record.Effective, Action: Action(record.Action),
		GrantIDs: append([]string(nil), record.GrantIDs...), DelegationID: record.DelegationID,
		CorrelationID: record.CorrelationID,
	}
	if err := validateDurableAuditEvent(event); err != nil {
		return AuditEvent{}, err
	}
	return event, nil
}

func validateDurableAuditEvent(event AuditEvent) error {
	if event.Outcome != "accepted" || strings.TrimSpace(event.Reason) != event.Reason || event.Reason == "" || len(event.Reason) > 128 {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(event.Audience.Node); err != nil ||
		(event.Audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR &&
			event.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION) ||
		event.Audience.ProtocolMajor != identitycontract.ProtocolMajor {
		return errInvalid
	}
	for _, principal := range []string{event.Principal, event.Actor, event.Effective} {
		if _, err := identityprincipal.Parse(principal); err != nil {
			return errInvalid
		}
	}
	if event.Actor != event.Principal || event.Effective == "" {
		return errInvalid
	}
	if _, err := identityprincipal.ParseDeviceID(event.DeviceID); err != nil {
		return errInvalid
	}
	if event.Action == applicationEnrollAction {
		if event.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION || event.Reason != "application_enrolled" {
			return errInvalid
		}
	} else if parsed, err := ParseAction(event.Audience.Interface, string(event.Action)); err != nil || parsed != event.Action {
		return errInvalid
	}
	for _, grantID := range event.GrantIDs {
		if !validArtifactID(grantID, identitycontract.AccessGrantPrefix) {
			return errInvalid
		}
	}
	if event.DelegationID != "" && !validArtifactID(event.DelegationID, identitycontract.DelegationPrefix) {
		return errInvalid
	}
	if !validAuditCorrelationID(event.CorrelationID) {
		return errInvalid
	}
	return nil
}

func successfulMutationEvent(call AuthorizedCall, reason string) AuditEvent {
	return AuditEvent{
		Outcome: "accepted", Reason: reason,
		Principal: call.actor, DeviceID: call.deviceID, Audience: call.audience,
		Actor: call.actor, Effective: call.effective, Action: call.action,
		GrantIDs: call.GrantIDs(), DelegationID: call.delegationID, CorrelationID: call.correlationID,
	}
}

func validAuditCorrelationID(value string) bool {
	if len(value) != len("c1_")+32 || !strings.HasPrefix(value, "c1_") {
		return false
	}
	for _, char := range value[len("c1_"):] {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func (s *Service) flushAuditOutbox(ctx context.Context) error {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	for {
		type pendingRecord struct {
			key []byte
			raw []byte
		}
		pending := make([]pendingRecord, 0)
		err := s.grants.database.View(ctx, func(tx storage.ReadTransaction) error {
			return tx.ForEach(auditOutboxBucket, func(key, raw []byte) error {
				if _, decodeErr := decodeAuditOutboxEvent(key, raw); decodeErr != nil {
					return decodeErr
				}
				pending = append(pending, pendingRecord{key: append([]byte(nil), key...), raw: append([]byte(nil), raw...)})
				return nil
			})
		})
		if err != nil {
			return fmt.Errorf("read identity audit outbox: %w", err)
		}
		if len(pending) == 0 || s.audit == nil {
			return nil
		}
		for _, record := range pending {
			event, err := decodeAuditOutboxEvent(record.key, record.raw)
			if err != nil {
				return fmt.Errorf("decode identity audit outbox: %w", err)
			}
			if durable, ok := s.audit.(DurableAuditSink); ok {
				if err := durable.RecordIdentityAccessDurable(event); err != nil {
					return fmt.Errorf("persist identity audit event: %w", err)
				}
			} else {
				s.audit.RecordIdentityAccess(event)
			}
			err = s.grants.database.Update(ctx, func(tx storage.WriteTransaction) error {
				current, found, readErr := tx.Get(auditOutboxBucket, record.key)
				if readErr != nil {
					return readErr
				}
				if !found {
					return nil
				}
				if !bytes.Equal(current, record.raw) {
					return errInvalid
				}
				return tx.Delete(auditOutboxBucket, record.key)
			})
			if err != nil {
				return fmt.Errorf("acknowledge identity audit outbox: %w", err)
			}
		}
	}
}
