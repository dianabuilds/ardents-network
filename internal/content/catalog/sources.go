package catalog

import (
	"fmt"
	"sort"
	"time"
)

const RemoteBlobSourceFreshnessTTL = 15 * time.Minute

type BlobIndex interface {
	Get(string) (Blob, bool)
}

type SourceIndex interface {
	List(string) []BlobSourceRecord
	Replace(string, []BlobSourceRecord)
}

func ObserveSource(blobs BlobIndex, sources SourceIndex, blobID string, source BlobSourceRecord) (BlobSourceRecord, error) {
	if _, ok := blobs.Get(blobID); !ok {
		return BlobSourceRecord{}, fmt.Errorf("blob not found")
	}
	source = NormalizeSource(blobID, source)
	items := append([]BlobSourceRecord(nil), sources.List(blobID)...)
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
	blobs BlobIndex,
	sources SourceIndex,
	blobID string,
	localNodeID string,
	now time.Time,
) []BlobSourceRecord {
	blob, ok := blobs.Get(blobID)
	if !ok {
		return nil
	}
	out := make([]BlobSourceRecord, 0, len(sources.List(blobID))+1)
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

func LocalBlobSource(blob Blob, localNodeID string) (BlobSourceRecord, bool) {
	if localNodeID == "" || !RequiresLocalPayload(blob.State) {
		return BlobSourceRecord{}, false
	}
	return BlobSourceRecord{
		BlobID:    blob.ID,
		NodeID:    localNodeID,
		Trust:     SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "local",
		Reason:    "blob is available on the local node",
	}, true
}

func NormalizeSource(blobID string, source BlobSourceRecord) BlobSourceRecord {
	source.BlobID = blobID
	if source.LastSeenAt.IsZero() {
		source.LastSeenAt = time.Now().UTC()
	}
	return source
}

func SameSource(left, right BlobSourceRecord) bool {
	return left.NodeID == right.NodeID &&
		left.ServiceID == right.ServiceID &&
		left.Transport == right.Transport
}

func CloneSource(source BlobSourceRecord) BlobSourceRecord {
	return BlobSourceRecord{
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

func ProjectSource(source BlobSourceRecord, now time.Time) BlobSourceRecord {
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

func sourceRequiresFreshObservation(source BlobSourceRecord) bool {
	if source.Transport == "" || source.Transport == "local" {
		return false
	}
	return !source.LastSeenAt.IsZero()
}
