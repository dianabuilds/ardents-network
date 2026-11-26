package route

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"slices"
	"time"
)

// InteriorSet retains the existing deterministic pair for one local role.
// Its original deadline survives persistence and cannot be renewed by failure.
type InteriorSet struct {
	Chosen, NotAfter time.Time
	Members          [2]Member
}

// SelectInterior preserves the accepted implementation's derivation from the
// durably random Entry pair. Changing that correlation requires a decision.
func SelectInterior(candidates []Member, entries [2]Member, now time.Time, domain uint8) (InteriorSet, error) {
	var eligible []Member
	for _, member := range candidates {
		if member.Domain == domain && now.Before(member.NotAfter) && !Conflict(entries[0], member) && !Conflict(entries[1], member) {
			eligible = append(eligible, member)
		}
	}
	slices.SortFunc(eligible, func(a, b Member) int { return bytes.Compare(a.NodeID[:], b.NodeID[:]) })
	var pairs [][2]Member
	for _, first := range eligible {
		for _, second := range eligible {
			if !Conflict(first, second) {
				pairs = append(pairs, [2]Member{first, second})
			}
		}
	}
	if now.IsZero() || len(pairs) == 0 {
		return InteriorSet{}, errors.New("two eligible Interior members unavailable")
	}
	transcript := []byte{domain}
	for _, member := range entries {
		transcript = append(transcript, member.NodeID[:]...)
		transcript = append(transcript, member.PublicKey[:]...)
	}
	digest := sha256.Sum256(transcript)
	index := binary.BigEndian.Uint64(digest[:8]) % uint64(len(pairs))
	set := InteriorSet{Chosen: now.UTC(), NotAfter: now.Add(30 * time.Minute).UTC(), Members: pairs[index]}
	for _, member := range set.Members {
		if member.NotAfter.Before(set.NotAfter) {
			set.NotAfter = member.NotAfter
		}
	}
	return set, nil
}

// Check retains the exact selected member and original horizon. It never
// redraws when a selected participant is unavailable.
func (set InteriorSet) Check(candidates []Member, entries [2]Member, now time.Time, domain uint8) error {
	if set.Chosen.IsZero() || now.Before(set.Chosen) || !now.Before(set.NotAfter) || set.NotAfter.After(set.Chosen.Add(30*time.Minute)) {
		return errors.New("interior set expired or time regressed")
	}
	if Conflict(set.Members[0], set.Members[1]) {
		return errors.New("interior pair conflicts")
	}
	for _, member := range set.Members {
		if member.Domain != domain || !now.Before(member.NotAfter) || Conflict(entries[0], member) || Conflict(entries[1], member) || !slices.Contains(candidates, member) {
			return errors.New("retained Interior member unavailable")
		}
	}
	return nil
}
