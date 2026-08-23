package entry

import (
	"errors"
	"time"
)

const maximumAdmissions = 64

type admissionRecord struct {
	InviteID        [32]byte `json:"invite_id"`
	AttachmentID    [32]byte `json:"attachment_id"`
	ClientKeyDigest [32]byte `json:"client_key_digest"`
	NotAfter        int64    `json:"not_after_unix"`
}

// AdmitterConfig supplies the narrow current Entry facts for an Initiator's
// durable one-use binding ledger. It uses a distinct owner-only Entry root
// from a User's local Entry set.
type AdmitterConfig struct {
	Root         string
	Verification Verification
}

// Admitter owns finite, durable replay outcomes for EntryBinding tuples. It
// retains no User identity, raw Invite, Route plan, or Service information.
type Admitter struct{ owner *owner }

// OpenAdmitter claims one Initiator-local Entry root and recovers its atomic
// replay ledger before accepting any new binding.
func OpenAdmitter(input AdmitterConfig) (*Admitter, error) {
	opened, err := Open(Config{Root: input.Root, Current: input.Verification.Current, Conflict: input.Verification.Conflict,
		Clock: input.Verification.Clock, TimeConfident: input.Verification.TimeConfident})
	if err != nil {
		return nil, err
	}
	return &Admitter{owner: opened}, nil
}

// Admit verifies one opaque Invite through Entry's current State/duty port and
// returns only the non-secret authorization required by Route.
func (value *Admitter) Admit(raw []byte) (Authorization, error) {
	if value == nil || value.owner == nil {
		return Authorization{}, errors.New("Entry Admitter is unavailable")
	}
	authorization, _, class, err := Verify(raw, Verification{Current: value.owner.config.Current, Conflict: value.owner.config.Conflict,
		Clock: value.owner.config.Clock, TimeConfident: value.owner.config.TimeConfident})
	if err != nil {
		return Authorization{}, err
	}
	if class != Accepted {
		return Authorization{}, errors.New("Entry Invite is not admitted")
	}
	return authorization, nil
}

// AdmitAndConsume rechecks the opaque Invite and writes its tuple while the
// same Entry owner lock is held. A State change cannot leave an independently
// verified authorization waiting to be committed by this Admitter.
func (value *Admitter) AdmitAndConsume(raw []byte, attachment, clientKey [32]byte, notAfter time.Time) (Authorization, error) {
	if value == nil || value.owner == nil {
		return Authorization{}, errors.New("Entry Admitter is unavailable")
	}
	value.owner.mu.Lock()
	defer value.owner.mu.Unlock()
	if value.owner.closed || value.owner.failed != nil {
		return Authorization{}, errors.New("Entry Admitter is unavailable")
	}
	decoded, _, class, err := validateInvite(raw, Verification{Current: value.owner.config.Current, Conflict: value.owner.config.Conflict,
		Clock: value.owner.config.Clock, TimeConfident: value.owner.config.TimeConfident})
	if err != nil {
		return Authorization{}, err
	}
	if class != Accepted {
		return Authorization{}, errors.New("Entry Invite is not admitted")
	}
	authorization := Authorization{InviteID: decoded.id, NetworkID: decoded.networkID, Digest: decoded.epochDigest,
		Epoch: decoded.epoch, InitiatorNodeID: decoded.nodeID, NotAfter: time.Unix(decoded.notAfter, 0).UTC()}
	if err := value.consumeLocked(authorization, attachment, clientKey, notAfter); err != nil {
		return Authorization{}, err
	}
	return authorization, nil
}

// Consume atomically records an authenticated EntryBinding replay tuple before
// Route allocates attachment work. The exact tuple is rejected after restart
// until its own bounded expiry.
func (value *Admitter) Consume(authorization Authorization, attachment, clientKey [32]byte, notAfter time.Time) error {
	if value == nil || value.owner == nil {
		return errors.New("Entry Admitter is unavailable")
	}
	value.owner.mu.Lock()
	defer value.owner.mu.Unlock()
	return value.consumeLocked(authorization, attachment, clientKey, notAfter)
}

func (value *Admitter) consumeLocked(authorization Authorization, attachment, clientKey [32]byte, notAfter time.Time) error {
	if value.owner.closed || value.owner.failed != nil || authorization.InviteID == [32]byte{} || attachment == [32]byte{} || clientKey == [32]byte{} ||
		notAfter.IsZero() || authorization.NotAfter.IsZero() || notAfter.After(authorization.NotAfter) {
		return errors.New("Entry admission tuple is invalid")
	}
	now := value.owner.config.Clock().UTC()
	if !value.owner.config.TimeConfident() || !now.Before(notAfter) {
		return errors.New("Entry admission tuple is expired")
	}
	next := value.owner.state.clone()
	next.Admissions = next.Admissions[:0]
	for _, record := range value.owner.state.Admissions {
		if now.Before(time.Unix(record.NotAfter, 0).UTC()) {
			next.Admissions = append(next.Admissions, record)
		}
	}
	for _, record := range next.Admissions {
		if record.InviteID == authorization.InviteID && record.AttachmentID == attachment && record.ClientKeyDigest == clientKey {
			return errors.New("Entry admission tuple was replayed")
		}
	}
	if len(next.Admissions) >= maximumAdmissions {
		return errors.New("Entry admission capacity is full")
	}
	next.Admissions = append(next.Admissions, admissionRecord{InviteID: authorization.InviteID, AttachmentID: attachment,
		ClientKeyDigest: clientKey, NotAfter: notAfter.UTC().Unix()})
	if err := value.owner.commit(next, false); err != nil {
		value.owner.failed = err
		return err
	}
	return nil
}

// Close releases the Admitter's exclusive durable root lease.
func (value *Admitter) Close() error {
	if value == nil || value.owner == nil {
		return nil
	}
	return value.owner.Close()
}
