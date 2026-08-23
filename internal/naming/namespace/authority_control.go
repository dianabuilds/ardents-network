package namespace

import (
	"bytes"
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
	store     *Store
}

// NewEvidenceControl installs the bounded volatile authority view retained by
// the Stage 6 evidence runner. Durable Gateways must use OpenControl and Submit
// so their accepted work enters the Namespace pending journal.
func NewEvidenceControl(network [32]byte, admission *Admission, order ClaimOrder,
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

// ApplyEvidence verifies one historical evidence operation in the volatile
// view. Its detailed result is deliberately unavailable to a durable Gateway.
func (control *control) ApplyEvidence(raw []byte, proof Proof) (string, uint64, uint64, []byte) {
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

// OpenControl restores one durable control chain. The control holds no
// caller-provided Record state: verified current records plus immutable pending
// successors are the only source from which it rebuilds its transition view.
func OpenControl(store *Store, admission *Admission, order ClaimOrder,
	clock func() time.Time, policy Policy,
) (*control, error) {
	if store == nil || store.root == nil || admission == nil || clock == nil {
		return nil, errors.New("name Authority control configuration is invalid")
	}
	control := &control{network: store.policy.Network, admission: admission, order: order,
		records: make(map[string]Record), clock: clock, policy: policy, store: store}
	if err := control.restore(); err != nil {
		return nil, err
	}
	return control, nil
}

// Submit records one opaque private control input as pending. It is never a
// current-state result: a threshold materialization remains the only route to
// an externally resolvable Name state.
func (control *control) Submit(submission Submission, proof Proof) string {
	control.mu.Lock()
	defer control.mu.Unlock()
	if control.store == nil {
		return "denied"
	}
	operation, err := decodeControlOperation(submission.raw)
	if err != nil || submission.digest != operation.OperationDigest || len(operation.SuccessorRecord) == 0 ||
		operation.Kind == "claim" ||
		proof.Challenge.Network != control.network || proof.Challenge.OperationDigest != operation.OperationDigest ||
		proof.Challenge.Surface != operation.surface() {
		return "denied"
	}
	now := control.clock()
	if accepted, _ := control.admission.Verify(now.UnixMilli(), proof); !accepted {
		return "denied"
	}
	updated, err := control.transitionAt(operation, now)
	if err != nil || !control.exactSignedSuccessor(updated, operation.SuccessorRecord) {
		return "denied"
	}
	if _, err := control.store.appendPending(submission.raw, operation.SuccessorRecord, now.UnixMilli()); err != nil {
		return "denied"
	}
	control.records[updated.Name] = updated
	return "submitted"
}

func (control *control) restore() error {
	current, _, err := control.store.root.load()
	if err != nil {
		return errors.New("name Authority control state is tampered")
	}
	cursor := uint64(0)
	if current != "" {
		snapshot, loadErr := control.store.load(0)
		if loadErr != nil {
			return loadErr
		}
		for _, signed := range snapshot.records {
			record, verifyErr := VerifyRecord(control.network, signed)
			if verifyErr != nil {
				return errors.New("name Authority current record is invalid")
			}
			control.records[record.Name] = record
		}
		cursor = snapshot.pending
	}
	entries, err := control.store.pending()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.sequence <= cursor {
			continue
		}
		operation, decodeErr := decodeControlOperation(entry.submission)
		if decodeErr != nil || len(operation.SuccessorRecord) == 0 ||
			!bytes.Equal(operation.SuccessorRecord, entry.successor) {
			return errors.New("name Authority pending submission is invalid")
		}
		updated, transitionErr := control.transitionAt(operation, time.UnixMilli(entry.decisionAt).UTC())
		if transitionErr != nil || !control.exactSignedSuccessor(updated, entry.successor) {
			return errors.New("name Authority pending chain is invalid")
		}
		control.records[updated.Name] = updated
	}
	return nil
}

func (control *control) exactSignedSuccessor(updated Record, signed []byte) bool {
	record, err := VerifyRecord(control.network, signed)
	if err != nil {
		return false
	}
	updatedWire, updatedErr := EncodeRecord(updated)
	recordWire, recordErr := EncodeRecord(record)
	return updatedErr == nil && recordErr == nil && bytes.Equal(updatedWire, recordWire)
}

func (control *control) transition(operation controlOperation) (Record, error) {
	return control.transitionAt(operation, control.clock())
}

func (control *control) transitionAt(operation controlOperation, now time.Time) (Record, error) {
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
	winner, err := OpenClaimWinner(control.order, proof)
	if err != nil {
		return Record{}, errors.New("root claim ordering is unavailable")
	}
	var predecessor *Record
	if exists {
		predecessor = &current
	}
	updated, err := winner.Materialize(predecessor, now, control.policy)
	if err != nil || updated.Name != operation.Name || updated.Authority != hex.EncodeToString(operation.Authority[:]) {
		return Record{}, errors.New("root claim materialization is unavailable")
	}
	return updated, nil
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
		return applyAt(&current, now, op, control.policy)
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
