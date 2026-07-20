package data

import (
	"fmt"
	"sort"
)

func (s *Service) SetReplicaIntent(intent ReplicaIntent) (ReplicaIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent = s.applyReplicaDefaults(intent)
	if err := s.validateReplicaIntentLocked(intent); err != nil {
		return ReplicaIntent{}, err
	}
	if existing, ok := s.availability.Intents[intent.ID]; ok {
		if intent.Version < existing.Version || (intent.Version == existing.Version && intent != existing) {
			return ReplicaIntent{}, fmt.Errorf("replica intent version conflicts with current state")
		}
	}
	s.availability.Intents[intent.ID] = intent
	return intent, s.saveLocked()
}

func (s *Service) applyReplicaDefaults(intent ReplicaIntent) ReplicaIntent {
	if intent.DesiredCopies == 0 {
		intent.DesiredCopies = s.cfg.DefaultDesiredReplicas
	}
	if intent.MinimumCopies == 0 {
		intent.MinimumCopies = s.cfg.DefaultMinimumReplicas
	}
	return intent
}

func (s *Service) ListReplicaIntents() []ReplicaIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ReplicaIntent, 0, len(s.availability.Intents))
	for _, intent := range s.availability.Intents {
		out = append(out, intent)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RootManifestID == out[j].RootManifestID {
			return out[i].Version < out[j].Version
		}
		return out[i].RootManifestID < out[j].RootManifestID
	})
	return out
}

func (s *Service) validateReplicaIntentLocked(intent ReplicaIntent) error {
	if intent.ID == "" || intent.RootManifestID == "" || intent.Version == 0 || intent.DesiredCopies <= 0 ||
		intent.MinimumCopies <= 0 || intent.MinimumCopies > intent.DesiredCopies || intent.LeaseDuration <= 0 ||
		intent.RenewalHorizon <= 0 || intent.RenewalHorizon >= intent.LeaseDuration || intent.CreatedAt.IsZero() || intent.UpdatedAt.IsZero() {
		return fmt.Errorf("replica intent is invalid")
	}
	if !intent.ExpiresAt.IsZero() && !intent.ExpiresAt.After(intent.UpdatedAt) {
		return fmt.Errorf("replica intent expiry is invalid")
	}
	if _, ok := s.manifests.Get(intent.RootManifestID); !ok {
		return fmt.Errorf("replica intent root manifest not found")
	}
	return nil
}

func (s *Service) intentForRootLocked(rootManifestID string) (ReplicaIntent, bool) {
	var selected ReplicaIntent
	found := false
	for _, intent := range s.availability.Intents {
		if intent.RootManifestID == rootManifestID && (!found || intent.Version > selected.Version) {
			selected, found = intent, true
		}
	}
	return selected, found
}

func (s *Service) resolveManifestBlobIDsLocked(rootID string) ([]string, error) {
	resolver := manifestBlobResolver{
		service: s, seenManifests: map[string]bool{}, active: map[string]bool{}, seenBlobs: map[string]bool{},
	}
	if err := resolver.visit(rootID); err != nil {
		return nil, err
	}
	if len(resolver.blobs) == 0 {
		return nil, fmt.Errorf("replica intent manifest has no blobs")
	}
	return resolver.blobs, nil
}

type manifestBlobResolver struct {
	service               *Service
	seenManifests, active map[string]bool
	seenBlobs             map[string]bool
	blobs                 []string
}

func (r *manifestBlobResolver) visit(id string) error {
	if r.active[id] {
		return fmt.Errorf("replica intent manifest graph is cyclic")
	}
	if r.seenManifests[id] {
		return nil
	}
	manifest, ok := r.service.manifests.Get(id)
	if !ok {
		return fmt.Errorf("replica intent manifest %q not found", id)
	}
	r.active[id] = true
	for _, ref := range manifest.Refs {
		if err := r.visitRef(ref); err != nil {
			return err
		}
	}
	delete(r.active, id)
	r.seenManifests[id] = true
	return nil
}

func (r *manifestBlobResolver) visitRef(ref Ref) error {
	switch ref.Kind {
	case "blob":
		if _, ok := r.service.blobs.Get(ref.ID); !ok {
			return fmt.Errorf("replica intent blob %q not found", ref.ID)
		}
		if !r.seenBlobs[ref.ID] {
			r.seenBlobs[ref.ID], r.blobs = true, append(r.blobs, ref.ID)
		}
		return nil
	case "manifest":
		return r.visit(ref.ID)
	default:
		return fmt.Errorf("replica intent manifest ref kind is invalid")
	}
}
