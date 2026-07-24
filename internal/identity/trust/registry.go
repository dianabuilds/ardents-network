// Package trust provides a closed, purpose-scoped registry of trusted Principals.
// It does not own authentication sessions or product authorization.
package trust

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"sort"

	"ardents/internal/identity/principal"
)

// Purpose is an exact trust purpose. Purposes are closed and do not imply one another.
type Purpose string

const (
	PurposeDiscoveryPublish Purpose = "discovery.publish"
	PurposeChannelIssue     Purpose = "channel.issue"
	PurposeIdentityAttest   Purpose = "identity.attest"
)

// Entry is one trusted Principal definition.
type Entry struct {
	Principal string
	PublicKey ed25519.PublicKey
	Purposes  []Purpose
}

// Generation is the full digest of a registry's canonical definitions.
type Generation [sha256.Size]byte

// String returns the complete lowercase hexadecimal digest.
func (generation Generation) String() string {
	return hex.EncodeToString(generation[:])
}

// Snapshot is a detached canonical view of a Registry.
type Snapshot struct {
	Generation Generation
	Entries    []Entry
}

// Registry is immutable after construction and safe for concurrent reads.
type Registry struct {
	byPurpose  map[Purpose]map[principal.ID]ed25519.PublicKey
	entries    []Entry
	generation Generation
}

// NewRegistry validates and copies all trust definitions.
func NewRegistry(entries []Entry) (*Registry, error) {
	registry := &Registry{byPurpose: make(map[Purpose]map[principal.ID]ed25519.PublicKey)}
	seenPrincipals := make(map[principal.ID]struct{}, len(entries))
	for _, entry := range entries {
		id, err := principal.Parse(entry.Principal)
		if err != nil {
			return nil, errors.New("trusted Principal is invalid")
		}
		if _, exists := seenPrincipals[id]; exists {
			return nil, errors.New("trusted Principal definition is duplicated")
		}
		seenPrincipals[id] = struct{}{}
		derived, err := principal.FromEd25519PublicKey(entry.PublicKey)
		if err != nil || !derived.Equal(id) {
			return nil, errors.New("trusted Principal public key does not match")
		}
		if len(entry.Purposes) == 0 {
			return nil, errors.New("trusted Principal has no purpose")
		}
		seenPurposes := make(map[Purpose]struct{}, len(entry.Purposes))
		for _, purpose := range entry.Purposes {
			if !purpose.valid() {
				return nil, errors.New("trust purpose is invalid")
			}
			if _, exists := seenPurposes[purpose]; exists {
				return nil, errors.New("trust purpose is duplicated")
			}
			seenPurposes[purpose] = struct{}{}
			if registry.byPurpose[purpose] == nil {
				registry.byPurpose[purpose] = make(map[principal.ID]ed25519.PublicKey)
			}
			registry.byPurpose[purpose][id] = clonePublicKey(entry.PublicKey)
		}
		registry.entries = append(registry.entries, cloneEntry(entry))
	}
	canonicalize(registry.entries)
	registry.generation = digestEntries(registry.entries)
	return registry, nil
}

// Lookup returns a detached public key only for an exact known purpose.
func (r *Registry) Lookup(purpose Purpose, id principal.ID) (ed25519.PublicKey, bool) {
	if r == nil || !purpose.valid() || id.String() == "" {
		return nil, false
	}
	key, ok := r.byPurpose[purpose][id]
	if !ok {
		return nil, false
	}
	return clonePublicKey(key), true
}

// Generation returns the digest identifying the complete canonical registry state.
func (r *Registry) Generation() Generation {
	if r == nil {
		return Generation{}
	}
	return r.generation
}

// Snapshot returns a deep copy in canonical order.
func (r *Registry) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	entries := make([]Entry, len(r.entries))
	for i := range r.entries {
		entries[i] = cloneEntry(r.entries[i])
	}
	return Snapshot{Generation: r.generation, Entries: entries}
}

func (purpose Purpose) valid() bool {
	switch purpose {
	case PurposeDiscoveryPublish, PurposeChannelIssue, PurposeIdentityAttest:
		return true
	default:
		return false
	}
}

func clonePublicKey(key ed25519.PublicKey) ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), key...)
}

func cloneEntry(entry Entry) Entry {
	return Entry{
		Principal: entry.Principal,
		PublicKey: clonePublicKey(entry.PublicKey),
		Purposes:  append([]Purpose(nil), entry.Purposes...),
	}
}

func canonicalize(entries []Entry) {
	for i := range entries {
		sort.Slice(entries[i].Purposes, func(left, right int) bool {
			return entries[i].Purposes[left] < entries[i].Purposes[right]
		})
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Principal < entries[right].Principal
	})
}

func digestEntries(entries []Entry) Generation {
	digest := sha256.New()
	_, _ = digest.Write([]byte("ardents:trusted-principal-registry:v1\x00"))
	writeUint32(digest, uint32(len(entries)))
	for _, entry := range entries {
		writeBytes(digest, []byte(entry.Principal))
		writeBytes(digest, entry.PublicKey)
		writeUint32(digest, uint32(len(entry.Purposes)))
		for _, purpose := range entry.Purposes {
			writeBytes(digest, []byte(purpose))
		}
	}
	var generation Generation
	copy(generation[:], digest.Sum(nil))
	return generation
}

func writeBytes(destination hash.Hash, value []byte) {
	writeUint32(destination, uint32(len(value)))
	_, _ = destination.Write(value)
}

func writeUint32(destination hash.Hash, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	_, _ = destination.Write(encoded[:])
}
