package duty

import (
	"errors"
	"path/filepath"
	"time"
)

// Open claims one initialized local-role root, or creates it only when Create
// is explicit. Every retained generation is verified before return.
func Open(input Config) (*store, error) {
	if input.Root == "" || input.Clock == nil || input.Clock().IsZero() {
		return nil, errors.New("local role configuration is incomplete")
	}
	root, err := filepath.Abs(input.Root)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(root, input.Create); err != nil {
		return nil, err
	}
	if err := verifyRootCandidate(root, input.Create); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := verifyRootClaim(root, input.Create); err != nil {
		return nil, err
	}
	if err := validateRootPermissions(root); err != nil {
		return nil, err
	}
	if err := prepareRoot(root, input.Create); err != nil {
		return nil, err
	}
	state, current, err := loadState(root)
	if err != nil {
		return nil, err
	}
	store := &store{root: root, clock: input.Clock, lease: lease, state: state, current: current}
	if current == "" {
		if !input.Create {
			return nil, errors.New("local role state is not initialized")
		}
		if err := store.commit(durableState{Duties: []dutyRecord{}}); err != nil {
			return nil, err
		}
	}
	opened = true
	return store, nil
}

// Replace atomically replaces one producer's complete current duty set.
func (store *store) Replace(producer [32]byte, duties []Duty) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || producer == ([32]byte{}) || len(duties) > 32 {
		if store.failed != nil {
			return errors.New("local role store requires restart after a failed commit")
		}
		return errors.New("local role replacement is invalid")
	}
	now := store.clock().UTC()
	next := durableState{Duties: make([]dutyRecord, 0, len(store.state.Duties)+len(duties)),
		TransitGrantSpends: liveTransitGrantSpends(store.state.TransitGrantSpends, now),
		TransitGrantIssuer: cloneTransitGrantIssuer(store.state.TransitGrantIssuer)}
	for _, retained := range store.state.Duties {
		if retained.Producer != producer && now.Unix() < retained.NotAfter {
			next.Duties = append(next.Duties, retained)
		}
	}
	for _, duty := range duties {
		if !validDuty(duty, now) {
			return errors.New("local role duty is invalid")
		}
		next.Duties = append(next.Duties, dutyRecord{Producer: producer, Identity: duty.Identity,
			Family: duty.Family, Class: duty.Class, State: duty.State, NotAfter: duty.NotAfter.Unix()})
	}
	if !validRecords(next.Duties) || !validTransitGrantSpends(next.TransitGrantSpends) || !validTransitGrantIssuer(next.TransitGrantIssuer) {
		return errors.New("local role state exceeds its bound")
	}
	return store.commit(next)
}

// SpendTransitGrant durably consumes one exact finite transit grant before a
// Node allocates route work. The root lease makes the check-and-write atomic
// across concurrent Node attempts and after process restart.
func (store *store) SpendTransitGrant(nodeID, grantID [32]byte, notAfter time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || nodeID == [32]byte{} || grantID == [32]byte{} || notAfter.IsZero() ||
		!notAfter.Equal(notAfter.UTC().Truncate(time.Second)) {
		return errors.New("transit grant spend is invalid")
	}
	now := store.clock().UTC()
	if !now.Before(notAfter) {
		return errors.New("transit grant is expired")
	}
	spends := liveTransitGrantSpends(store.state.TransitGrantSpends, now)
	for _, spend := range spends {
		if spend.GrantID == grantID {
			return errors.New("transit grant was already spent")
		}
	}
	if len(spends) >= maximumTransitGrantSpends {
		return errors.New("transit grant spend ledger is full")
	}
	next := durableState{Duties: make([]dutyRecord, 0, len(store.state.Duties)), TransitGrantSpends: spends,
		TransitGrantIssuer: cloneTransitGrantIssuer(store.state.TransitGrantIssuer)}
	for _, retained := range store.state.Duties {
		if now.Unix() < retained.NotAfter {
			next.Duties = append(next.Duties, retained)
		}
	}
	next.TransitGrantSpends = append(next.TransitGrantSpends, transitGrantSpend{NodeID: nodeID, GrantID: grantID, NotAfter: notAfter.Unix()})
	if !validRecords(next.Duties) || !validTransitGrantSpends(next.TransitGrantSpends) || !validTransitGrantIssuer(next.TransitGrantIssuer) {
		return errors.New("transit grant spend ledger is invalid")
	}
	return store.commit(next)
}

func liveTransitGrantSpends(spends []transitGrantSpend, now time.Time) []transitGrantSpend {
	result := make([]transitGrantSpend, 0, len(spends))
	for _, spend := range spends {
		if now.Unix() < spend.NotAfter {
			result = append(result, spend)
		}
	}
	return result
}

// Remove atomically removes every duty owned by producer.
func (store *store) Remove(producer [32]byte) error { return store.Replace(producer, nil) }

// Conflict reads the held current generation and reports one non-Initiator,
// unexpired identity or family collision.
func (store *store) Conflict(identity, family [32]byte) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil {
		if store.failed != nil {
			return false, errors.New("local role store requires restart after a failed commit")
		}
		return false, errors.New("local role store is closed")
	}
	now := store.clock().UTC().Unix()
	for _, duty := range store.state.Duties {
		if now < duty.NotAfter && duty.Class != "ordinary-initiator" &&
			(identity != ([32]byte{}) && duty.Identity == identity || family != ([32]byte{}) && duty.Family == family) {
			return true, nil
		}
	}
	return false, nil
}

// Close releases the exclusive root lease. It is idempotent.
func (store *store) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return nil
	}
	store.closed = true
	return store.lease.release()
}

func validDuty(duty Duty, now time.Time) bool {
	return duty.Identity != ([32]byte{}) && duty.Family != ([32]byte{}) &&
		validClass(duty.Class) && validState(duty.State) && now.Before(duty.NotAfter)
}

func validClass(value string) bool {
	switch value {
	case "ordinary-initiator", "direct-source", "route-interior", "route-rendezvous",
		"route-responder", "route-introduction", "destination-resolution", "node-duty":
		return true
	default:
		return false
	}
}

func validState(value string) bool {
	return value == "exposed" || value == "prepared" || value == "quarantined" || value == "live"
}
