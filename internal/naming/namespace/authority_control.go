package namespace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// control owns authenticated naming transitions behind one private Gateway.
// It receives canonical naming-side bytes and no Relay or Endpoint identity.
type control struct {
	mu        sync.Mutex
	network   [32]byte
	admission *Admission
	order     ClaimOrder
	records   map[string]Record
	clock     func() time.Time
	policy    Policy
}

// NewControl installs one bounded authority state view for a Gateway.
func NewControl(network [32]byte, admission *Admission, order ClaimOrder,
	records []Record, clock func() time.Time, policy Policy,
) (*control, error) {
	if network == [32]byte{} || admission == nil || clock == nil {
		return nil, errors.New("name Authority control configuration is invalid")
	}
	values := make(map[string]Record, len(records))
	for _, record := range records {
		if _, exists := values[record.Name]; exists {
			return nil, errors.New("name Authority control state is duplicated")
		}
		if _, err := EncodeRecord(record); err != nil {
			return nil, errors.New("name Authority control state is invalid")
		}
		values[record.Name] = record
	}
	return &control{network: network, admission: admission, order: order,
		records: values, clock: clock, policy: policy}, nil
}

// Apply verifies one submission into the Gateway's volatile pending chain.
// Its detailed result is local to this implementation; Resolution exposes it
// only as a non-current submission outcome.
func (control *control) Apply(raw []byte, proof Proof) (string, uint64, uint64, []byte) {
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
	state, err := EncodeRecord(updated)
	if err != nil {
		return deniedControl()
	}
	control.records[updated.Name] = updated
	return "accepted", updated.Generation, updated.Revision, state
}

func (control *control) transition(operation controlOperation) (Record, error) {
	now := control.clock()
	current, exists := control.records[operation.Name]
	switch operation.Kind {
	case "claim":
		return control.claim(operation, current, exists, now)
	case "delegate":
		return control.delegate(operation, now)
	case "renew", "record", "release", "transfer", "policy", "recovery":
		if !exists {
			return Record{}, errors.New("name Authority predecessor is unavailable")
		}
		return control.existing(operation, current, now)
	default:
		return Record{}, errors.New("name Authority operation is unavailable")
	}
}

func (control *control) claim(operation controlOperation, current Record, exists bool,
	now time.Time,
) (Record, error) {
	var proof ClaimProof
	_, decodeErr := CanonicalProof(operation.OrderingProof, &proof)
	if decodeErr != nil {
		return Record{}, errors.New("root claim proof is invalid")
	}
	result, err := control.order.Verify(proof)
	if err != nil || result.Outcome != "accepted" {
		return Record{}, errors.New("root claim ordering is unavailable")
	}
	op := Op{Kind: "claim", Name: operation.Name, Generation: operation.Generation,
		ClaimOrdinal: result.WinnerOrdinal, Authority: hex.EncodeToString(operation.Authority[:]),
		LeaseDuration: durationUntil(now, operation.LeaseNotAfter)}
	if !exists {
		return ApplyOrderedClaim(control.order, proof, nil, now.Unix(), op, control.policy)
	}
	return ApplyOrderedClaim(control.order, proof, &current, now.Unix(), op, control.policy)
}

func (control *control) delegate(operation controlOperation, now time.Time) (Record, error) {
	parent, exists := control.records[operation.ParentName]
	if !exists || parent.Generation != operation.ParentGeneration || parent.Revision != operation.ParentRevision {
		return Record{}, errors.New("parent authority predecessor is unavailable")
	}
	parents, err := control.lineage(parent, now)
	if err != nil {
		return Record{}, err
	}
	parents = append([]Record{parent}, parents...)
	op := Op{Kind: "claim", Name: operation.Name, Generation: operation.ChildGeneration,
		Authority: hex.EncodeToString(operation.Authority[:]), Parents: parents,
		LeaseDuration: durationUntil(now, operation.LeaseNotAfter)}
	return applyTransition(control.network, parent, op, operation.AuthorityProof, now.Unix(), control.policy)
}

func (control *control) existing(operation controlOperation, current Record,
	now time.Time,
) (Record, error) {
	op, threshold, err := operation.lifecycle(control.network, current, now)
	if err != nil {
		return Record{}, err
	}
	op.Parents, err = control.lineage(current, now)
	if err != nil {
		return Record{}, err
	}
	if threshold {
		return ApplyAt(&current, now, op, control.policy)
	}
	return applyTransition(control.network, current, op, operation.AuthorityProof, now.Unix(), control.policy)
}

// lineage returns the authoritative immediate-parent-to-root chain for record.
// The control holds its lock while resolving it, so callers cannot provide a
// stale or substituted Record graph between predecessor verification and
// transition application.
func (control *control) lineage(record Record, now time.Time) ([]Record, error) {
	parents := make([]Record, 0, 4)
	seen := map[string]bool{record.Name: true}
	for current := record; current.ParentName != ""; {
		if len(parents) >= 126 || seen[current.ParentName] {
			return nil, errors.New("name Authority parent lineage is invalid")
		}
		parent, exists := control.records[current.ParentName]
		if !exists || parent.Generation != current.ParentGeneration {
			return nil, errors.New("name Authority parent lineage is unavailable")
		}
		if err := validateRecord(parent); err != nil {
			return nil, errors.New("name Authority parent lineage is invalid")
		}
		parents, seen[parent.Name] = append(parents, parent), true
		current = parent
	}
	if _, err := validateParents(record.Name, parents, now.Unix()); err != nil {
		return nil, errors.New("name Authority parent lineage is not resolvable")
	}
	return parents, nil
}

func durationUntil(now time.Time, notAfter int64) time.Duration {
	return time.Duration(notAfter-now.UnixMilli()) * time.Millisecond
}

func deniedControl() (string, uint64, uint64, []byte) { return "denied", 0, 0, nil }

func controlOperationDigest(raw []byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}
