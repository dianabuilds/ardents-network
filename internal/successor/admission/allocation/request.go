package allocation

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

var errInvalid = errors.New("admission allocation is invalid")

// Request is a copied, holder-verified request for one aligned hour. The zero
// value cannot decide an allocation; callers cannot supply accepted fields.
type Request struct {
	request admission.PermissionRequest
	digest  [32]byte
	valid   bool
}

// Prepare verifies public input before Custody asks for a password. The Network
// comes from its independently selected Authority binding, not this request.
func Prepare(raw []byte, network [32]byte, now time.Time) (Request, error) {
	request, err := admission.DecodePermissionRequest(raw)
	if err != nil || request.Permission.NetworkID != network || !currentHour(request.Permission, now) {
		return Request{}, errInvalid
	}
	return Request{request: request, digest: sha256.Sum256(raw), valid: true}, nil
}

// Decide validates the unlocked authority's key and current journal and hour.
// The result is a policy decision only: Custody must commit its exact journal
// through encrypted publication, readback and floor before returning a signature.
func (request Request) Decide(journal []byte, authority [32]byte, now time.Time) (Decision, error) {
	if !request.valid || request.request.AuthorityKey != authority || !currentHour(request.request.Permission, now) {
		return Decision{}, errInvalid
	}
	entries, err := decodeJournal(journal)
	if err != nil {
		return Decision{}, err
	}
	window := uint64(request.request.Permission.NotBefore.Unix())
	if windowRegressed(entries, window) {
		return Decision{}, errInvalid
	}
	entries = forWindow(entries, window)
	for _, entry := range entries {
		if entry.id != request.request.Permission.PermissionID {
			continue
		}
		if entry.digest != request.digest {
			return Decision{}, errInvalid
		}
		return Decision{permission: request.request.Permission, repeated: true}, nil
	}
	entry := reservation{window: window, role: request.request.Role, tokens: tokenTotal(request.request.Permission.Maxima),
		id: request.request.Permission.PermissionID, digest: request.digest}
	next, err := encodeJournal(append(entries, entry))
	if err != nil {
		return Decision{}, err
	}
	return Decision{permission: request.request.Permission, journal: next}, nil
}

func currentHour(permission admission.Permission, now time.Time) bool {
	window := now.UTC().Truncate(time.Hour)
	return permission.NotBefore.Equal(window) && permission.NotAfter.Equal(window.Add(time.Hour))
}

// Decision contains an exact unsigned permission and, for new allocations, the
// candidate journal. It has no signing capability or durable confirmation.
type Decision struct {
	permission admission.Permission
	journal    []byte
	repeated   bool
}

func (decision Decision) Permission() admission.Permission { return decision.permission }
func (decision Decision) Repeated() bool                   { return decision.repeated }

// Journal returns a copy so callers cannot change a retained decision.
func (decision Decision) Journal() []byte { return bytes.Clone(decision.journal) }
