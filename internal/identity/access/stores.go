package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"sync"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
)

type storedChallenge struct {
	challenge Challenge
	source    SourceKey
}

type rateState struct {
	tokens  float64
	updated time.Time
}

type challengeStore struct {
	mu           sync.Mutex
	items        map[ChallengeID]storedChallenge
	sourceCounts map[SourceKey]int
	rates        map[SourceKey]rateState
}

func newChallengeStore() *challengeStore {
	return &challengeStore{items: map[ChallengeID]storedChallenge{}, sourceCounts: map[SourceKey]int{}, rates: map[SourceKey]rateState{}}
}

func (s *challengeStore) add(now time.Time, item storedChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if len(s.items) >= identitycontract.MaxActiveChallenges || s.sourceCounts[item.source] >= identitycontract.MaxActiveChallengesPerSource {
		return ErrResourceExhausted
	}
	state, exists := s.rates[item.source]
	if !exists {
		state = rateState{tokens: identitycontract.BeginRateBurst, updated: now}
	}
	state = refillRate(state, now)
	if state.tokens < 1 {
		s.rates[item.source] = state
		return ErrResourceExhausted
	}
	state.tokens--
	s.rates[item.source] = state
	if _, exists := s.items[item.challenge.ID]; exists {
		return ErrInternal
	}
	s.items[item.challenge.ID] = item
	s.sourceCounts[item.source]++
	return nil
}

func refillRate(state rateState, now time.Time) rateState {
	if now.After(state.updated) {
		state.tokens += now.Sub(state.updated).Minutes() * identitycontract.BeginRatePerMinute
		if state.tokens > identitycontract.BeginRateBurst {
			state.tokens = identitycontract.BeginRateBurst
		}
		state.updated = now
	}
	return state
}

func (s *challengeStore) get(now time.Time, id ChallengeID) (storedChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	item, ok := s.items[id]
	return item, ok
}

func (s *challengeStore) consume(now time.Time, expected storedChallenge) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	actual, ok := s.items[expected.challenge.ID]
	if !ok || actual != expected {
		return false
	}
	s.deleteLocked(expected.challenge.ID, actual)
	return true
}

func (s *challengeStore) cleanupLocked(now time.Time) {
	for id, item := range s.items {
		if !now.Before(item.challenge.ExpiresAt) {
			s.deleteLocked(id, item)
		}
	}
	for source, state := range s.rates {
		state = refillRate(state, now)
		if s.sourceCounts[source] == 0 && state.tokens >= identitycontract.BeginRateBurst {
			delete(s.rates, source)
		} else {
			s.rates[source] = state
		}
	}
}

func (s *challengeStore) deleteLocked(id ChallengeID, item storedChallenge) {
	delete(s.items, id)
	s.sourceCounts[item.source]--
	if s.sourceCounts[item.source] == 0 {
		delete(s.sourceCounts, item.source)
	}
}

type sessionGroup struct {
	source    SourceKey
	principal string
	audience  Audience
}
type storedSession struct {
	session Session
	group   sessionGroup
}

type sessionStore struct {
	mu           sync.Mutex
	key          [32]byte
	items        map[[32]byte]storedSession
	groups       map[sessionGroup]int
	byDevice     map[string]map[[32]byte]struct{}
	byCredential map[string]map[[32]byte]struct{}
}

func newSessionStore(key [32]byte) *sessionStore {
	return &sessionStore{key: key, items: map[[32]byte]storedSession{}, groups: map[sessionGroup]int{}, byDevice: map[string]map[[32]byte]struct{}{}, byCredential: map[string]map[[32]byte]struct{}{}}
}

func (s *sessionStore) lookup(secret SessionSecret) [32]byte {
	mac := hmac.New(sha256.New, s.key[:])
	_, _ = mac.Write(secret[:])
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}

func (s *sessionStore) add(now time.Time, secret SessionSecret, session Session, source SourceKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	group := sessionGroup{source: source, principal: session.Principal, audience: session.Binding.Audience}
	if len(s.items) >= identitycontract.MaxActiveSessions || s.groups[group] >= identitycontract.MaxActiveSessionsPerSourceKey {
		return ErrResourceExhausted
	}
	lookup := s.lookup(secret)
	if _, exists := s.items[lookup]; exists {
		return ErrInternal
	}
	s.items[lookup] = storedSession{session: session, group: group}
	s.groups[group]++
	addReverse(s.byDevice, session.DeviceID, lookup)
	addReverse(s.byCredential, session.CredentialID, lookup)
	return nil
}

func (s *sessionStore) get(now time.Time, secret SessionSecret) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	entry, ok := s.items[s.lookup(secret)]
	return entry.session, ok
}

func (s *sessionStore) invalidateDevice(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for lookup := range s.byDevice[deviceID] {
		if entry, ok := s.items[lookup]; ok {
			s.deleteLocked(lookup, entry)
		}
	}
}

func (s *sessionStore) cleanupLocked(now time.Time) {
	for lookup, entry := range s.items {
		if !now.Before(entry.session.ExpiresAt) {
			s.deleteLocked(lookup, entry)
		}
	}
}

func (s *sessionStore) deleteLocked(lookup [32]byte, entry storedSession) {
	delete(s.items, lookup)
	s.groups[entry.group]--
	if s.groups[entry.group] == 0 {
		delete(s.groups, entry.group)
	}
	removeReverse(s.byDevice, entry.session.DeviceID, lookup)
	removeReverse(s.byCredential, entry.session.CredentialID, lookup)
}

func addReverse(index map[string]map[[32]byte]struct{}, key string, lookup [32]byte) {
	if index[key] == nil {
		index[key] = map[[32]byte]struct{}{}
	}
	index[key][lookup] = struct{}{}
}
func removeReverse(index map[string]map[[32]byte]struct{}, key string, lookup [32]byte) {
	delete(index[key], lookup)
	if len(index[key]) == 0 {
		delete(index, key)
	}
}

type proofEntry struct {
	digest    [32]byte
	expiresAt time.Time
}
type proofStore struct {
	mu    sync.Mutex
	key   [32]byte
	items map[[32]byte]proofEntry
}

func newProofStore(key [32]byte) *proofStore {
	return &proofStore{key: key, items: map[[32]byte]proofEntry{}}
}
func (s *proofStore) digest(proof EnrollmentProof) [32]byte {
	mac := hmac.New(sha256.New, s.key[:])
	_, _ = mac.Write(proof[:])
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out
}
func (s *proofStore) add(now time.Time, proof EnrollmentProof, bindingDigest [32]byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup(now)
	if len(s.items) >= identitycontract.MaxActiveChallenges {
		return ErrResourceExhausted
	}
	key := s.digest(proof)
	if _, ok := s.items[key]; ok {
		return ErrInternal
	}
	s.items[key] = proofEntry{digest: bindingDigest, expiresAt: expiresAt}
	return nil
}
func (s *proofStore) consume(now time.Time, proof EnrollmentProof, bindingDigest [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup(now)
	key := s.digest(proof)
	entry, ok := s.items[key]
	if !ok || !hmac.Equal(entry.digest[:], bindingDigest[:]) {
		return false
	}
	delete(s.items, key)
	return true
}
func (s *proofStore) cleanup(now time.Time) {
	for key, entry := range s.items {
		if !now.Before(entry.expiresAt) {
			delete(s.items, key)
		}
	}
}
