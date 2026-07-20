package observed

import (
	"fmt"
	"sort"

	model "ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
)

type LocalPayloadPresence func(id string) bool

func ObjectPartState(nodeState string, objects map[string]model.Object, manifests map[string]model.Manifest, blobs map[string]model.Blob) (string, string) {
	if nodeState != "ready" {
		return nodeState, ""
	}
	missing, sample := countMissingMetadataBlobRefs(objects, manifests, blobs)
	if missing == 0 {
		return "ready", ""
	}
	return "degraded", formatBrokenRefReason(sample, missing)
}

func BlobPartState(nodeState string, blobs map[string]model.Blob, hasLocalPayload LocalPayloadPresence) (string, string) {
	if nodeState != "ready" {
		return nodeState, ""
	}
	missing, sample := countMissingLocalPayloads(blobs, hasLocalPayload)
	if missing == 0 {
		return "ready", ""
	}
	return "degraded", formatMissingPayloadReason(sample, missing)
}

func countMissingMetadataBlobRefs(objects map[string]model.Object, manifests map[string]model.Manifest, blobs map[string]model.Blob) (int, string) {
	count := 0
	sample := ""
	for _, id := range sortedObjectIDs(objects) {
		object := objects[id]
		for _, ref := range object.BlobRefs {
			if ref.Kind != "blob" || ref.ID == "" {
				continue
			}
			if _, ok := blobs[ref.ID]; ok {
				continue
			}
			count++
			if sample == "" {
				sample = fmt.Sprintf("object %q references missing blob %q", id, ref.ID)
			}
		}
	}
	for _, id := range sortedManifestIDs(manifests) {
		manifest := manifests[id]
		for _, ref := range manifest.Refs {
			if ref.Kind != "blob" || ref.ID == "" {
				continue
			}
			if _, ok := blobs[ref.ID]; ok {
				continue
			}
			count++
			if sample == "" {
				sample = fmt.Sprintf("manifest %q references missing blob %q", id, ref.ID)
			}
		}
	}
	return count, sample
}

func countMissingLocalPayloads(blobs map[string]model.Blob, hasLocalPayload LocalPayloadPresence) (int, string) {
	count := 0
	sample := ""
	for _, id := range sortedBlobIDs(blobs) {
		blob := blobs[id]
		if !datapayload.StateRequiresLocalPayload(blob.State) || hasLocalPayload(id) {
			continue
		}
		count++
		if sample == "" {
			sample = fmt.Sprintf("blob %q is %q without local payload", id, blob.State)
		}
	}
	return count, sample
}

func sortedObjectIDs(items map[string]model.Object) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedBlobIDs(items map[string]model.Blob) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedManifestIDs(items map[string]model.Manifest) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func formatBrokenRefReason(sample string, count int) string {
	if count <= 1 {
		return sample
	}
	return fmt.Sprintf("%s (%d broken blob refs total)", sample, count)
}

func formatMissingPayloadReason(sample string, count int) string {
	if count <= 1 {
		return sample
	}
	return fmt.Sprintf("%s (%d local payloads missing total)", sample, count)
}
