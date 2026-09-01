package credential

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	issuerRootMarkerName       = ".ardents-local-roles-v2"
	issuerRootMarker           = "ardents-local-roles-v2\n"
	legacyIssuerRootMarkerName = ".ardents-local-roles-v1"
	issuerRootLockName         = ".ardents-local-roles-lock"
	maximumIssuerState         = 64 << 10
	maximumIssuerMaterial      = 256
	maximumIssuerProfile       = 4096
	maximumIssuerBudget        = 64
)

var issuerStateName = regexp.MustCompile(`^[0-9a-f]{64}$`)

type issuerRootStore struct {
	mu      sync.Mutex
	root    string
	clock   func() time.Time
	lease   issuerRootLease
	state   issuerRootState
	current string
	closed  bool
	failed  error
}

type issuerRootState struct {
	Version            uint8             `json:"version"`
	Generation         uint64            `json:"generation"`
	Previous           string            `json:"previous,omitempty"`
	Duties             []json.RawMessage `json:"duties"`
	TransitGrantSpends []json.RawMessage `json:"transit_grant_spends"`
	TransitGrantIssuer *issuerRootRecord `json:"transit_grant_issuer,omitempty"`
}

type issuerRootRecord struct {
	StateGeneration string                  `json:"state_generation"`
	ProfileDigest   [32]byte                `json:"profile_digest,omitempty"`
	Profile         []byte                  `json:"profile,omitempty"`
	NetworkID       [32]byte                `json:"network_id"`
	Digest          [32]byte                `json:"digest"`
	IssuerNodeID    [32]byte                `json:"issuer_node_id"`
	GrantSignerID   [32]byte                `json:"grant_signer_id"`
	Epoch           uint64                  `json:"epoch"`
	NotAfter        int64                   `json:"not_after"`
	Budget          uint16                  `json:"budget"`
	Withdrawn       bool                    `json:"withdrawn"`
	PrivateMaterial []byte                  `json:"private_material"`
	Reservations    []issuerRootReservation `json:"reservations"`
}

type issuerRootReservation struct {
	RequestID     [32]byte `json:"request_id"`
	RequestDigest [32]byte `json:"request_digest"`
	GrantID       [32]byte `json:"grant_id"`
}

func openIssuerRootStore(root string, clock func() time.Time, create bool) (*issuerRootStore, error) {
	if root == "" || clock == nil || clock().IsZero() {
		return nil, errors.New("transit grant issuer root configuration is incomplete")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := inspectIssuerRoot(absolute, create); err != nil {
		return nil, err
	}
	if err := preflightIssuerRoot(absolute); err != nil {
		return nil, err
	}
	lease, err := acquireIssuerRootLease(absolute)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := preflightIssuerRoot(absolute); err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if err := validateIssuerRootPermissions(absolute, info); err != nil {
		return nil, err
	}
	if err := prepareIssuerRoot(absolute, create); err != nil {
		return nil, err
	}
	state, current, err := loadIssuerRootState(absolute)
	if err != nil {
		return nil, err
	}
	store := &issuerRootStore{root: absolute, clock: clock, lease: lease, state: state, current: current}
	if current == "" {
		if !create {
			return nil, errors.New("transit grant issuer root is not initialized")
		}
		if err := store.commit(issuerRootState{Duties: []json.RawMessage{}, TransitGrantSpends: []json.RawMessage{}}); err != nil {
			return nil, err
		}
	}
	opened = true
	return store, nil
}

func (store *issuerRootStore) initialize(profileDigest [32]byte, budget uint16, privateMaterial, profile []byte) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || profileDigest == [32]byte{} || budget == 0 || budget > maximumIssuerBudget ||
		len(privateMaterial) == 0 || len(privateMaterial) > maximumIssuerMaterial || len(profile) == 0 || len(profile) > maximumIssuerProfile ||
		sha256.Sum256(profile) != profileDigest {
		return nil, errors.New("transit grant issuer root initialization is invalid")
	}
	if existing := store.state.TransitGrantIssuer; existing != nil {
		if existing.Budget != budget {
			return nil, errors.New("transit grant issuer root cannot be replaced")
		}
		return append([]byte(nil), existing.Profile...), nil
	}
	next := cloneIssuerRootState(store.state)
	next.TransitGrantIssuer = &issuerRootRecord{ProfileDigest: profileDigest, Profile: append([]byte(nil), profile...), Budget: budget,
		PrivateMaterial: append([]byte(nil), privateMaterial...), Reservations: []issuerRootReservation{}}
	if err := store.commit(next); err != nil {
		return nil, err
	}
	return append([]byte(nil), profile...), nil
}

func (store *issuerRootStore) material() ([]byte, []byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	issuer := store.state.TransitGrantIssuer
	if store.closed || store.failed != nil || issuer == nil || len(issuer.PrivateMaterial) == 0 || len(issuer.Profile) == 0 {
		return nil, nil, errors.New("transit grant issuer root is unavailable")
	}
	return append([]byte(nil), issuer.PrivateMaterial...), append([]byte(nil), issuer.Profile...), nil
}

func (store *issuerRootStore) bind(scope issuerScope) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validIssuerScope(scope) {
		return errors.New("transit grant issuer binding is invalid")
	}
	issuer := store.state.TransitGrantIssuer
	if issuer == nil {
		return errors.New("transit grant issuer root is not initialized")
	}
	if issuer.Epoch != 0 {
		if sameIssuerScope(issuer, scope) {
			return nil
		}
		return errors.New("transit grant issuer root is already bound")
	}
	next := cloneIssuerRootState(store.state)
	next.TransitGrantIssuer.StateGeneration = scope.Generation
	next.TransitGrantIssuer.NetworkID, next.TransitGrantIssuer.Digest = scope.NetworkID, scope.Digest
	next.TransitGrantIssuer.IssuerNodeID, next.TransitGrantIssuer.GrantSignerID = scope.IssuerNodeID, scope.GrantSignerID
	next.TransitGrantIssuer.Epoch, next.TransitGrantIssuer.NotAfter = scope.Epoch, scope.NotAfter.Unix()
	return store.commit(next)
}

func (store *issuerRootStore) find(scope issuerScope, requestID, requestDigest [32]byte) ([32]byte, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validIssuerScope(scope) || requestID == [32]byte{} || requestDigest == [32]byte{} {
		return [32]byte{}, false, errors.New("transit grant reservation lookup is invalid")
	}
	issuer := store.state.TransitGrantIssuer
	if !sameIssuerScope(issuer, scope) {
		return [32]byte{}, false, errors.New("transit grant issuer scope is unavailable")
	}
	for _, reservation := range issuer.Reservations {
		if reservation.RequestID == requestID {
			if reservation.RequestDigest != requestDigest {
				return [32]byte{}, false, errIssuerRequestConflict
			}
			return reservation.GrantID, true, nil
		}
	}
	return [32]byte{}, false, nil
}

func (store *issuerRootStore) reserve(scope issuerScope, requestID, requestDigest, grantID [32]byte) ([32]byte, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !validIssuerScope(scope) || requestID == [32]byte{} || requestDigest == [32]byte{} || grantID == [32]byte{} {
		return [32]byte{}, false, errors.New("transit grant reservation is invalid")
	}
	issuer := store.state.TransitGrantIssuer
	if !sameIssuerScope(issuer, scope) {
		return [32]byte{}, false, errors.New("transit grant issuer scope is unavailable")
	}
	for _, reservation := range issuer.Reservations {
		if reservation.RequestID == requestID {
			if reservation.RequestDigest != requestDigest {
				return [32]byte{}, false, errIssuerRequestConflict
			}
			return reservation.GrantID, false, nil
		}
	}
	if issuer.Withdrawn || !store.clock().UTC().Before(scope.NotAfter) {
		return [32]byte{}, false, errIssuerWithdrawn
	}
	if len(issuer.Reservations) >= int(issuer.Budget) {
		return [32]byte{}, false, errIssuerExhausted
	}
	next := cloneIssuerRootState(store.state)
	next.TransitGrantIssuer.Reservations = append(next.TransitGrantIssuer.Reservations, issuerRootReservation{
		RequestID: requestID, RequestDigest: requestDigest, GrantID: grantID})
	if err := store.commit(next); err != nil {
		return [32]byte{}, false, err
	}
	return grantID, true, nil
}

func (store *issuerRootStore) withdraw(scope issuerScope) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || !sameIssuerScope(store.state.TransitGrantIssuer, scope) {
		return errors.New("transit grant issuer withdrawal is invalid")
	}
	if store.state.TransitGrantIssuer.Withdrawn {
		return nil
	}
	next := cloneIssuerRootState(store.state)
	next.TransitGrantIssuer.Withdrawn = true
	return store.commit(next)
}

func (store *issuerRootStore) close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return nil
	}
	store.closed = true
	return store.lease.release()
}

func (store *issuerRootStore) commit(next issuerRootState) error {
	next.Version, next.Generation, next.Previous = 2, store.state.Generation+1, store.current
	if !validIssuerRootState(next) {
		return errors.New("transit grant issuer root state is invalid")
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumIssuerState {
		return errors.New("transit grant issuer root exceeds its byte bound")
	}
	name := issuerStateDigest(raw)
	if err := writeIssuerGeneration(store.root, name, raw); err != nil {
		store.failed = err
		return err
	}
	if err := replaceIssuerFile(store.root, "watermark", "watermark", fmt.Sprintf("%d %s\n", next.Generation, name)); err != nil {
		store.failed = err
		return err
	}
	if err := replaceIssuerFile(store.root, "current", "current", name+"\n"); err != nil {
		store.failed = err
		return err
	}
	if err := cleanupIssuerGenerations(store.root, name, store.current); err != nil {
		store.failed = err
		return err
	}
	store.state, store.current = next, name
	return nil
}

func loadIssuerRootState(root string) (issuerRootState, string, error) {
	generation, watermark, hasWatermark, err := loadIssuerWatermark(root)
	if err != nil {
		return issuerRootState{}, "", err
	}
	pointer, err := readIssuerFile(filepath.Join(root, "current"), 65)
	if os.IsNotExist(err) {
		if !hasWatermark {
			return issuerRootState{Version: 2, Duties: []json.RawMessage{}, TransitGrantSpends: []json.RawMessage{}}, "", nil
		}
		state, loadErr := loadIssuerGeneration(root, watermark)
		if loadErr != nil || state.Generation != generation {
			return issuerRootState{}, "", errors.New("transit grant issuer watermark target is invalid")
		}
		if err := replaceIssuerFile(root, "current", "current", watermark+"\n"); err != nil {
			return issuerRootState{}, "", err
		}
		return state, watermark, nil
	}
	if err != nil {
		return issuerRootState{}, "", err
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !issuerStateName.MatchString(current) {
		return issuerRootState{}, "", errors.New("transit grant issuer state pointer is invalid")
	}
	state, err := loadIssuerGeneration(root, current)
	if err != nil {
		return issuerRootState{}, "", err
	}
	if !hasWatermark || state.Generation > generation || state.Generation == generation && current != watermark {
		return issuerRootState{}, "", errors.New("transit grant issuer state violates its watermark")
	}
	if state.Generation < generation {
		state, err = loadIssuerGeneration(root, watermark)
		if err != nil || state.Generation != generation {
			return issuerRootState{}, "", errors.New("transit grant issuer watermark target is invalid")
		}
		current = watermark
		if err := replaceIssuerFile(root, "current", "current", current+"\n"); err != nil {
			return issuerRootState{}, "", err
		}
	}
	return state, current, nil
}

func loadIssuerGeneration(root, name string) (issuerRootState, error) {
	raw, err := readIssuerFile(filepath.Join(root, "state-"+name), maximumIssuerState)
	if err != nil || issuerStateDigest(raw) != name {
		return issuerRootState{}, errors.New("transit grant issuer generation is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var state issuerRootState
	if decoder.Decode(&state) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validIssuerRootState(state) {
		return issuerRootState{}, errors.New("transit grant issuer generation is invalid")
	}
	return state, nil
}

func loadIssuerWatermark(root string) (uint64, string, bool, error) {
	raw, err := readIssuerFile(filepath.Join(root, "watermark"), 96)
	if os.IsNotExist(err) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	line := strings.TrimSuffix(string(raw), "\n")
	parts := strings.Split(line, " ")
	if len(parts) != 2 || string(raw) != line+"\n" || !issuerStateName.MatchString(parts[1]) {
		return 0, "", false, errors.New("transit grant issuer watermark is invalid")
	}
	generation, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || generation == 0 || fmt.Sprintf("%d %s\n", generation, parts[1]) != string(raw) {
		return 0, "", false, errors.New("transit grant issuer watermark is invalid")
	}
	return generation, parts[1], true, nil
}

func validIssuerRootState(state issuerRootState) bool {
	if state.Version != 2 || state.Generation == 0 || state.Previous != "" && !issuerStateName.MatchString(state.Previous) ||
		state.Duties == nil || len(state.Duties) != 0 || state.TransitGrantSpends == nil || len(state.TransitGrantSpends) != 0 {
		return false
	}
	issuer := state.TransitGrantIssuer
	if issuer == nil {
		return true
	}
	profileValid := len(issuer.Profile) > 0 && len(issuer.Profile) <= maximumIssuerProfile && sha256.Sum256(issuer.Profile) == issuer.ProfileDigest
	bound := issuerStateName.MatchString(issuer.StateGeneration) && issuer.NetworkID != [32]byte{} && issuer.Digest != [32]byte{} && issuer.IssuerNodeID != [32]byte{} &&
		issuer.GrantSignerID != [32]byte{} && issuer.Epoch != 0 && issuer.NotAfter > 0
	unbound := issuer.StateGeneration == "" && issuer.NetworkID == [32]byte{} && issuer.Digest == [32]byte{} && issuer.IssuerNodeID == [32]byte{} &&
		issuer.GrantSignerID == [32]byte{} && issuer.Epoch == 0 && issuer.NotAfter == 0
	if !profileValid || !bound && !unbound || issuer.Budget == 0 || issuer.Budget > maximumIssuerBudget ||
		len(issuer.PrivateMaterial) == 0 || len(issuer.PrivateMaterial) > maximumIssuerMaterial || len(issuer.Reservations) > int(issuer.Budget) {
		return false
	}
	seen := make(map[[32]byte]bool, len(issuer.Reservations))
	for _, reservation := range issuer.Reservations {
		if reservation.RequestID == [32]byte{} || reservation.RequestDigest == [32]byte{} || reservation.GrantID == [32]byte{} || seen[reservation.RequestID] {
			return false
		}
		seen[reservation.RequestID] = true
	}
	return true
}

func cloneIssuerRootState(state issuerRootState) issuerRootState {
	result := state
	result.Duties = append([]json.RawMessage{}, state.Duties...)
	result.TransitGrantSpends = append([]json.RawMessage{}, state.TransitGrantSpends...)
	if state.TransitGrantIssuer != nil {
		issuer := *state.TransitGrantIssuer
		issuer.Profile = append([]byte(nil), issuer.Profile...)
		issuer.PrivateMaterial = append([]byte(nil), issuer.PrivateMaterial...)
		issuer.Reservations = append([]issuerRootReservation(nil), issuer.Reservations...)
		result.TransitGrantIssuer = &issuer
	}
	return result
}

func issuerStateDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
