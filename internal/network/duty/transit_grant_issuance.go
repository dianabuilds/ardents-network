package duty

import (
	"errors"
	"time"
)

// InitializeTransitGrantIssuer fixes one immutable finite budget for this
// local-role root. Reopening the exact scope and budget is idempotent; changing
// either requires a distinct root/duty generation.
func (store *store) InitializeTransitGrantIssuer(scope TransitGrantIssuerScope, budget uint16, privateMaterial []byte) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validTransitGrantIssuerScope(scope) || budget == 0 || budget > maximumTransitGrantBudget ||
		len(privateMaterial) == 0 || len(privateMaterial) > 256 {
		return nil, errors.New("transit grant issuer initialization is invalid")
	}
	if existing := store.state.TransitGrantIssuer; existing != nil {
		if sameTransitGrantIssuerScope(existing, scope) && existing.Budget == budget {
			return append([]byte(nil), existing.PrivateMaterial...), nil
		}
		return nil, errors.New("transit grant issuer scope or budget cannot be replaced")
	}
	next := cloneDurableState(store.state)
	next.TransitGrantIssuer = &transitGrantIssuer{NetworkID: scope.NetworkID, Digest: scope.Digest,
		IssuerNodeID: scope.IssuerNodeID, GrantSignerID: scope.GrantSignerID, Epoch: scope.Epoch,
		NotAfter: scope.NotAfter.Unix(), Budget: budget, PrivateMaterial: append([]byte(nil), privateMaterial...),
		Reservations: []transitGrantReservation{}}
	if err := store.commit(next); err != nil {
		return nil, err
	}
	return append([]byte(nil), privateMaterial...), nil
}

// ReserveTransitGrant returns the durable Grant ID for one exact Request ID.
// The boolean is true only for the first reservation; a byte-identical replay
// returns the prior ID without spending another budget unit.
func (store *store) ReserveTransitGrant(scope TransitGrantIssuerScope, requestID, requestDigest, grantID [32]byte) ([32]byte, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validTransitGrantIssuerScope(scope) || requestID == [32]byte{} ||
		requestDigest == [32]byte{} || grantID == [32]byte{} {
		return [32]byte{}, false, errors.New("transit grant reservation is invalid")
	}
	issuer := store.state.TransitGrantIssuer
	if issuer == nil || !sameTransitGrantIssuerScope(issuer, scope) {
		return [32]byte{}, false, errors.New("transit grant issuer scope is unavailable")
	}
	for _, reservation := range issuer.Reservations {
		if reservation.RequestID != requestID {
			continue
		}
		if reservation.RequestDigest != requestDigest {
			return [32]byte{}, false, ErrTransitGrantRequestConflict
		}
		return reservation.GrantID, false, nil
	}
	if issuer.Withdrawn || !store.clock().UTC().Before(time.Unix(issuer.NotAfter, 0).UTC()) {
		return [32]byte{}, false, ErrTransitGrantIssuerWithdrawn
	}
	if len(issuer.Reservations) >= int(issuer.Budget) {
		return [32]byte{}, false, ErrTransitGrantIssuerExhausted
	}
	next := cloneDurableState(store.state)
	next.TransitGrantIssuer.Reservations = append(next.TransitGrantIssuer.Reservations, transitGrantReservation{
		RequestID: requestID, RequestDigest: requestDigest, GrantID: grantID})
	if err := store.commit(next); err != nil {
		return [32]byte{}, false, err
	}
	return grantID, true, nil
}

// FindTransitGrantReservation looks up only a previously accepted exact
// Request ID. It never creates a reservation or consumes budget.
func (store *store) FindTransitGrantReservation(scope TransitGrantIssuerScope, requestID, requestDigest [32]byte) ([32]byte, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validTransitGrantIssuerScope(scope) || requestID == [32]byte{} || requestDigest == [32]byte{} {
		return [32]byte{}, false, errors.New("transit grant reservation lookup is invalid")
	}
	issuer := store.state.TransitGrantIssuer
	if issuer == nil || !sameTransitGrantIssuerScope(issuer, scope) {
		return [32]byte{}, false, errors.New("transit grant issuer scope is unavailable")
	}
	for _, reservation := range issuer.Reservations {
		if reservation.RequestID != requestID {
			continue
		}
		if reservation.RequestDigest != requestDigest {
			return [32]byte{}, false, ErrTransitGrantRequestConflict
		}
		return reservation.GrantID, true, nil
	}
	return [32]byte{}, false, nil
}

// WithdrawTransitGrantIssuer irreversibly prevents new reservations while
// retaining exact prior Request-ID reconciliation until duty expiry.
func (store *store) WithdrawTransitGrantIssuer(scope TransitGrantIssuerScope) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validTransitGrantIssuerScope(scope) || store.state.TransitGrantIssuer == nil ||
		!sameTransitGrantIssuerScope(store.state.TransitGrantIssuer, scope) {
		return errors.New("transit grant issuer withdrawal is invalid")
	}
	if store.state.TransitGrantIssuer.Withdrawn {
		return nil
	}
	next := cloneDurableState(store.state)
	next.TransitGrantIssuer.Withdrawn = true
	return store.commit(next)
}

func validTransitGrantIssuerScope(scope TransitGrantIssuerScope) bool {
	return scope.NetworkID != [32]byte{} && scope.Digest != [32]byte{} && scope.IssuerNodeID != [32]byte{} &&
		scope.GrantSignerID != [32]byte{} && scope.Epoch != 0 && !scope.NotAfter.IsZero() && scope.NotAfter.Unix() > 0 &&
		scope.NotAfter.Equal(scope.NotAfter.UTC().Truncate(time.Second))
}

func sameTransitGrantIssuerScope(issuer *transitGrantIssuer, scope TransitGrantIssuerScope) bool {
	return issuer != nil && issuer.NetworkID == scope.NetworkID && issuer.Digest == scope.Digest &&
		issuer.IssuerNodeID == scope.IssuerNodeID && issuer.GrantSignerID == scope.GrantSignerID && issuer.Epoch == scope.Epoch &&
		issuer.NotAfter == scope.NotAfter.Unix()
}

func cloneDurableState(state durableState) durableState {
	result := state
	result.Duties = append([]dutyRecord(nil), state.Duties...)
	result.TransitGrantSpends = append([]transitGrantSpend(nil), state.TransitGrantSpends...)
	result.TransitGrantIssuer = cloneTransitGrantIssuer(state.TransitGrantIssuer)
	return result
}

func cloneTransitGrantIssuer(issuer *transitGrantIssuer) *transitGrantIssuer {
	if issuer == nil {
		return nil
	}
	result := *issuer
	result.PrivateMaterial = append([]byte(nil), issuer.PrivateMaterial...)
	result.Reservations = append([]transitGrantReservation(nil), issuer.Reservations...)
	return &result
}
