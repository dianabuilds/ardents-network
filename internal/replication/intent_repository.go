package replication

import (
	"fmt"

	model "ardents/internal/content/catalog"
	"ardents/internal/replication/availability"
)

func (r *Repository) SetReplicaIntent(intent availability.ReplicaIntent) (availability.ReplicaIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if intent.DesiredCopies == 0 {
		intent.DesiredCopies = r.desired
	}
	if intent.MinimumCopies == 0 {
		intent.MinimumCopies = r.minimum
	}
	if err := r.validateReplicaIntentLocked(intent); err != nil {
		return availability.ReplicaIntent{}, err
	}
	if existing, ok := r.availability.Intents[intent.ID]; ok {
		if intent.Version < existing.Version || (intent.Version == existing.Version && intent != existing) {
			return availability.ReplicaIntent{}, fmt.Errorf("replica intent version conflicts with current state")
		}
	}
	r.availability.Intents[intent.ID] = intent
	return intent, r.saveLocked()
}

func (r *Repository) validateReplicaIntentLocked(intent availability.ReplicaIntent) error {
	if intent.ID == "" || intent.RootManifestID == "" || intent.Version == 0 || intent.DesiredCopies <= 0 ||
		intent.MinimumCopies <= 0 || intent.MinimumCopies > intent.DesiredCopies || intent.LeaseDuration <= 0 ||
		intent.RenewalHorizon <= 0 || intent.RenewalHorizon >= intent.LeaseDuration || intent.CreatedAt.IsZero() || intent.UpdatedAt.IsZero() {
		return fmt.Errorf("replica intent is invalid")
	}
	if !intent.ExpiresAt.IsZero() && !intent.ExpiresAt.After(intent.UpdatedAt) {
		return fmt.Errorf("replica intent expiry is invalid")
	}
	if _, ok := r.content.ReadTransferManifest(intent.RootManifestID); !ok {
		return fmt.Errorf("replica intent root manifest not found")
	}
	return nil
}

func (r *Repository) intentForRootLocked(rootManifestID string) (availability.ReplicaIntent, bool) {
	var selected availability.ReplicaIntent
	found := false
	for _, intent := range r.availability.Intents {
		if intent.RootManifestID == rootManifestID && (!found || intent.Version > selected.Version) {
			selected, found = intent, true
		}
	}
	return selected, found
}

func (r *Repository) resolveManifestBlobIDsLocked(rootID string) ([]string, error) {
	resolver := manifestBlobResolver{
		repository: r, seenManifests: map[string]bool{}, active: map[string]bool{}, seenBlobs: map[string]bool{},
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
	repository            *Repository
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
	manifest, ok := r.repository.content.ReadTransferManifest(id)
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

func (r *manifestBlobResolver) visitRef(ref model.Ref) error {
	switch ref.Kind {
	case "blob":
		if _, ok := r.repository.content.GetBlob(ref.ID); !ok {
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
