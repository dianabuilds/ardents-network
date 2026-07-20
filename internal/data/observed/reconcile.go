package observed

import (
	"time"

	model "ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
)

type RemovePayload func(id string) error

func ReconcileLoadedBlobs(blobs map[string]model.Blob, now time.Time, hasLocalPayload LocalPayloadPresence, removePayload RemovePayload) (map[string]model.Blob, bool, error) {
	updated := cloneBlobs(blobs)
	changed, err := pruneExpired(updated, now.UTC(), removePayload)
	if err != nil {
		return nil, false, err
	}
	if reconcileMissingPayload(updated, hasLocalPayload) {
		changed = true
	}
	if !changed {
		return blobs, false, nil
	}
	return updated, true, nil
}

func pruneExpired(blobs map[string]model.Blob, now time.Time, removePayload RemovePayload) (bool, error) {
	changed := false
	for id, blob := range blobs {
		if blob.State != "retained-temporary" || blob.ExpiresAt.IsZero() || blob.ExpiresAt.After(now) {
			continue
		}
		if err := removePayload(id); err != nil {
			return false, err
		}
		blob.State = "expired"
		blobs[id] = blob
		changed = true
	}
	return changed, nil
}

func reconcileMissingPayload(blobs map[string]model.Blob, hasLocalPayload LocalPayloadPresence) bool {
	changed := false
	for id, blob := range blobs {
		if !datapayload.StateRequiresLocalPayload(blob.State) || hasLocalPayload(id) {
			continue
		}
		blob.State = "deleted"
		blob.ExpiresAt = time.Time{}
		blobs[id] = blob
		changed = true
	}
	return changed
}

func cloneBlobs(items map[string]model.Blob) map[string]model.Blob {
	cloned := make(map[string]model.Blob, len(items))
	for id, blob := range items {
		cloned[id] = blob
	}
	return cloned
}
