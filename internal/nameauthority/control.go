package nameauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
	"github.com/dianabuilds/ardents-network/internal/nameclaim"
	"github.com/dianabuilds/ardents-network/internal/namelease"
)

// control owns authenticated naming transitions behind one private Gateway.
// It receives canonical naming-side bytes and no Relay or Endpoint identity.
type control struct {
	mu        sync.Mutex
	network   [32]byte
	admission *nameadmission.Admission
	order     nameclaim.ClaimOrder
	records   map[string]namelease.Record
	clock     func() time.Time
	policy    namelease.Policy
}

// NewControl installs one bounded authority state view for a Gateway.
func NewControl(network [32]byte, admission *nameadmission.Admission, order nameclaim.ClaimOrder,
	records []namelease.Record, clock func() time.Time, policy namelease.Policy,
) (*control, error) {
	if network == [32]byte{} || admission == nil || clock == nil {
		return nil, errors.New("name Authority control configuration is invalid")
	}
	values := make(map[string]namelease.Record, len(records))
	for _, record := range records {
		if _, exists := values[record.Name]; exists {
			return nil, errors.New("name Authority control state is duplicated")
		}
		if _, err := namelease.EncodeRecord(record); err != nil {
			return nil, errors.New("name Authority control state is invalid")
		}
		values[record.Name] = record
	}
	return &control{network: network, admission: admission, order: order,
		records: values, clock: clock, policy: policy}, nil
}

// Apply verifies anonymous admission, authority or threshold proof, and the
// exact predecessor before committing one in-memory transition result.
func (control *control) Apply(raw []byte, proof nameadmission.Proof) (string, uint64, uint64, []byte) {
	control.mu.Lock()
	defer control.mu.Unlock()
	operation, err := decodeControlOperation(raw)
	if err != nil || proof.Challenge.Network != control.network ||
		proof.Challenge.OperationDigest != operation.OperationDigest ||
		proof.Challenge.Surface != operation.surface() {
		return deniedControl()
	}
	if accepted, _ := control.admission.Verify(control.clock().UnixMilli(), proof); !accepted {
		return deniedControl()
	}
	updated, err := control.transition(operation)
	if err != nil {
		return deniedControl()
	}
	state, err := namelease.EncodeRecord(updated)
	if err != nil {
		return deniedControl()
	}
	control.records[updated.Name] = updated
	return "accepted", updated.Generation, updated.Revision, state
}

func (control *control) transition(operation controlOperation) (namelease.Record, error) {
	now := control.clock()
	current, exists := control.records[operation.Name]
	switch operation.Kind {
	case "claim":
		return control.claim(operation, current, exists, now)
	case "delegate":
		return control.delegate(operation, now)
	case "renew", "record", "release", "transfer", "policy", "recovery":
		if !exists {
			return namelease.Record{}, errors.New("name Authority predecessor is unavailable")
		}
		return control.existing(operation, current, now)
	default:
		return namelease.Record{}, errors.New("name Authority operation is unavailable")
	}
}

func (control *control) claim(operation controlOperation, current namelease.Record, exists bool,
	now time.Time,
) (namelease.Record, error) {
	var proof nameclaim.Proof
	_, decodeErr := nameclaim.CanonicalProof(operation.OrderingProof, &proof)
	if decodeErr != nil {
		return namelease.Record{}, errors.New("root claim proof is invalid")
	}
	result, err := control.order.Verify(proof)
	if err != nil || result.Outcome != "accepted" {
		return namelease.Record{}, errors.New("root claim ordering is unavailable")
	}
	op := namelease.Op{Kind: "claim", Name: operation.Name, Generation: operation.Generation,
		ClaimOrdinal: result.WinnerOrdinal, Authority: hex.EncodeToString(operation.Authority[:]),
		LeaseDuration: durationUntil(now, operation.LeaseNotAfter)}
	if !exists {
		return ApplyOrderedClaim(control.order, proof, nil, now.Unix(), op, control.policy)
	}
	return ApplyOrderedClaim(control.order, proof, &current, now.Unix(), op, control.policy)
}

func (control *control) delegate(operation controlOperation, now time.Time) (namelease.Record, error) {
	parent, exists := control.records[operation.ParentName]
	if !exists || parent.Generation != operation.ParentGeneration || parent.Revision != operation.ParentRevision {
		return namelease.Record{}, errors.New("parent authority predecessor is unavailable")
	}
	op := namelease.Op{Kind: "claim", Name: operation.Name, Generation: operation.ChildGeneration,
		Authority: hex.EncodeToString(operation.Authority[:]), Parents: []namelease.Record{parent},
		LeaseDuration: durationUntil(now, operation.LeaseNotAfter)}
	return applyTransition(control.network, parent, op, operation.AuthorityProof, now.Unix(), control.policy)
}

func (control *control) existing(operation controlOperation, current namelease.Record,
	now time.Time,
) (namelease.Record, error) {
	op, threshold, err := operation.lifecycle(control.network, current, now)
	if err != nil {
		return namelease.Record{}, err
	}
	if threshold {
		return namelease.Apply(&current, now.Unix(), op, control.policy)
	}
	return applyTransition(control.network, current, op, operation.AuthorityProof, now.Unix(), control.policy)
}

func durationUntil(now time.Time, notAfter int64) time.Duration {
	return time.Duration(notAfter-now.UnixMilli()) * time.Millisecond
}

func deniedControl() (string, uint64, uint64, []byte) { return "denied", 0, 0, nil }

func controlOperationDigest(raw []byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}
