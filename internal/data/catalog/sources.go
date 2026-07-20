package catalog

import (
	"fmt"
	"sort"
	"time"

	"ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
	statepkg "ardents/internal/data/state"
)

const RemoteBlobSourceFreshnessTTL = 15 * time.Minute

func ObserveSource(blobs *statepkg.BlobStore, sources *statepkg.SourceLedger, blobID string, source model.BlobSourceRecord) (model.BlobSourceRecord, error) {
	if _, ok := blobs.Get(blobID); !ok {
		return model.BlobSourceRecord{}, fmt.Errorf("blob not found")
	}
	source = NormalizeSource(blobID, source)
	items := append([]model.BlobSourceRecord(nil), sources.List(blobID)...)
	replaced := false
	for i, item := range items {
		if SameSource(item, source) {
			items[i] = source
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, source)
	}
	sources.Replace(blobID, items)
	return CloneSource(source), nil
}

func ListSources(
	blobs *statepkg.BlobStore,
	sources *statepkg.SourceLedger,
	blobID string,
	localNodeID string,
	now time.Time,
) []model.BlobSourceRecord {
	blob, ok := blobs.Get(blobID)
	if !ok {
		return nil
	}
	out := make([]model.BlobSourceRecord, 0, len(sources.List(blobID))+1)
	if local, ok := LocalBlobSource(blob, localNodeID); ok {
		out = append(out, local)
	}
	for _, item := range sources.List(blobID) {
		out = append(out, ProjectSource(item, now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Transport != out[j].Transport {
			return out[i].Transport < out[j].Transport
		}
		if out[i].NodeID != out[j].NodeID {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].ServiceID < out[j].ServiceID
	})
	return out
}

func LocalBlobSource(blob model.Blob, localNodeID string) (model.BlobSourceRecord, bool) {
	if localNodeID == "" || !datapayload.StateRequiresLocalPayload(blob.State) {
		return model.BlobSourceRecord{}, false
	}
	return model.BlobSourceRecord{
		BlobID:    blob.ID,
		NodeID:    localNodeID,
		Trust:     model.SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "local",
		Reason:    "blob is available on the local node",
	}, true
}

func NormalizeSource(blobID string, source model.BlobSourceRecord) model.BlobSourceRecord {
	source.BlobID = blobID
	if source.LastSeenAt.IsZero() {
		source.LastSeenAt = time.Now().UTC()
	}
	return source
}

func SameSource(left, right model.BlobSourceRecord) bool {
	return left.NodeID == right.NodeID &&
		left.ServiceID == right.ServiceID &&
		left.Transport == right.Transport
}

func CloneSource(source model.BlobSourceRecord) model.BlobSourceRecord {
	return model.BlobSourceRecord{
		BlobID:     source.BlobID,
		NodeID:     source.NodeID,
		ServiceID:  source.ServiceID,
		Trust:      source.Trust,
		Usable:     source.Usable,
		Transport:  source.Transport,
		LastSeenAt: source.LastSeenAt,
		Reason:     source.Reason,
	}
}

func ProjectSource(source model.BlobSourceRecord, now time.Time) model.BlobSourceRecord {
	out := CloneSource(source)
	if !sourceRequiresFreshObservation(out) {
		return out
	}
	if now.Sub(out.LastSeenAt) <= RemoteBlobSourceFreshnessTTL {
		return out
	}
	out.Usable = false
	out.Trust.Usable = false
	out.Reason = fmt.Sprintf(
		"remote source observation is stale; last seen at %s",
		out.LastSeenAt.UTC().Format(time.RFC3339),
	)
	return out
}

func sourceRequiresFreshObservation(source model.BlobSourceRecord) bool {
	if source.Transport == "" || source.Transport == "local" {
		return false
	}
	return !source.LastSeenAt.IsZero()
}
