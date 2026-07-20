package lifecycle

import (
	"fmt"
	"sort"
	"time"

	model "ardents/internal/data/model"
	statepkg "ardents/internal/data/state"
)

func Start(ledger *statepkg.TransferLedger, record model.TransferRecord) model.TransferRecord {
	record = Normalize(record)
	ledger.Put(record)
	return Clone(record)
}

func Get(ledger *statepkg.TransferLedger, id string) (model.TransferRecord, bool) {
	item, ok := ledger.Get(id)
	if !ok {
		return model.TransferRecord{}, false
	}
	return Clone(item), true
}

func List(ledger *statepkg.TransferLedger) []model.TransferRecord {
	out := make([]model.TransferRecord, 0, ledger.Count())
	for _, item := range ledger.Items {
		out = append(out, Clone(item))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.After(out[j].StartedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func Finish(
	ledger *statepkg.TransferLedger,
	id string,
	state string,
	peer string,
	totalBytes int64,
	reason string,
) (model.TransferRecord, error) {
	item, ok := ledger.Get(id)
	if !ok {
		return model.TransferRecord{}, fmt.Errorf("transfer not found")
	}
	now := time.Now().UTC()
	item.State = state
	item.Peer = firstNonEmpty(peer, item.Peer)
	item.Reason = firstNonEmpty(reason, item.Reason)
	item.UpdatedAt = now
	item.FinishedAt = &now
	if totalBytes > 0 {
		item.TotalBytes = totalBytes
		item.ProgressBytes = totalBytes
	}
	ledger.Put(item)
	return Clone(item), nil
}

func Update(ledger *statepkg.TransferLedger, id string, progressBytes, totalBytes int64, reason string) (model.TransferRecord, error) {
	item, ok := ledger.Get(id)
	if !ok {
		return model.TransferRecord{}, fmt.Errorf("transfer not found")
	}
	if progressBytes < item.ProgressBytes || totalBytes < progressBytes {
		return model.TransferRecord{}, fmt.Errorf("transfer progress is invalid")
	}
	item.ProgressBytes = progressBytes
	item.TotalBytes = totalBytes
	item.Reason = firstNonEmpty(reason, item.Reason)
	item.UpdatedAt = time.Now().UTC()
	ledger.Put(item)
	return Clone(item), nil
}

func Normalize(record model.TransferRecord) model.TransferRecord {
	now := time.Now().UTC()
	if record.ID == "" {
		record.ID = fmt.Sprintf("xfer-%d", now.UnixNano())
	}
	if record.State == "" {
		record.State = "pending"
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.StartedAt
	}
	return record
}

func Clone(record model.TransferRecord) model.TransferRecord {
	out := record
	if record.FinishedAt != nil {
		finished := *record.FinishedAt
		out.FinishedAt = &finished
	}
	return out
}

func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
