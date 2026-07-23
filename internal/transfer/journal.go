package transfer

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"ardents/internal/storage"
)

type Record struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind,omitempty"`
	ResourceID    string     `json:"resource_id,omitempty"`
	Direction     string     `json:"direction,omitempty"`
	State         string     `json:"state"`
	ProgressBytes int64      `json:"progress_bytes,omitempty"`
	TotalBytes    int64      `json:"total_bytes,omitempty"`
	Peer          string     `json:"peer,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Reason        string     `json:"reason,omitempty"`
}

type Journal struct {
	mu    sync.Mutex
	path  string
	items map[string]Record
}

func NewJournal(path string) *Journal {
	return &Journal{path: path, items: map[string]Record{}}
}

func (j *Journal) Load() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	var snapshot struct {
		Transfers map[string]Record `json:"transfers"`
	}
	found, err := storage.LoadJSON(j.path, "transfer", "history", &snapshot)
	if err != nil {
		return err
	}
	if snapshot.Transfers == nil {
		snapshot.Transfers = map[string]Record{}
	}
	j.items = cloneRecords(snapshot.Transfers)
	if !found {
		return j.saveLocked()
	}
	return nil
}

func (j *Journal) Start(record Record) (Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record = normalizeRecord(record)
	if _, exists := j.items[record.ID]; exists {
		return Record{}, fmt.Errorf("transfer %s already exists", record.ID)
	}
	j.items[record.ID] = cloneRecord(record)
	if err := j.saveLocked(); err != nil {
		delete(j.items, record.ID)
		return Record{}, err
	}
	return cloneRecord(record), nil
}

func (j *Journal) Progress(id string, progressBytes, totalBytes int64, reason string) (Record, error) {
	return j.change(id, func(record *Record) error {
		if progressBytes < record.ProgressBytes || totalBytes < progressBytes {
			return fmt.Errorf("transfer progress is invalid")
		}
		record.ProgressBytes = progressBytes
		record.TotalBytes = totalBytes
		record.Reason = firstNonEmpty(reason, record.Reason)
		record.UpdatedAt = time.Now().UTC()
		return nil
	})
}

func (j *Journal) Complete(id, peer string, totalBytes int64, reason string) (Record, error) {
	return j.finish(id, "completed", peer, totalBytes, reason)
}

func (j *Journal) Fail(id, peer, reason string) (Record, error) {
	return j.finish(id, "failed", peer, 0, reason)
}

func (j *Journal) Get(id string) (Record, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	record, ok := j.items[id]
	return cloneRecord(record), ok
}

func (j *Journal) List() []Record {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Record, 0, len(j.items))
	for _, record := range j.items {
		out = append(out, cloneRecord(record))
	}
	sort.Slice(out, func(i, k int) bool {
		if !out[i].StartedAt.Equal(out[k].StartedAt) {
			return out[i].StartedAt.After(out[k].StartedAt)
		}
		return out[i].ID < out[k].ID
	})
	return out
}

func (j *Journal) finish(id, state, peer string, totalBytes int64, reason string) (Record, error) {
	return j.change(id, func(record *Record) error {
		now := time.Now().UTC()
		record.State = state
		record.Peer = firstNonEmpty(peer, record.Peer)
		record.Reason = firstNonEmpty(reason, record.Reason)
		record.UpdatedAt = now
		record.FinishedAt = &now
		if totalBytes > 0 {
			record.TotalBytes = totalBytes
			record.ProgressBytes = totalBytes
		}
		return nil
	})
}

func (j *Journal) change(id string, mutate func(*Record) error) (Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	current, ok := j.items[id]
	if !ok {
		return Record{}, fmt.Errorf("transfer not found")
	}
	updated := cloneRecord(current)
	if err := mutate(&updated); err != nil {
		return Record{}, err
	}
	j.items[id] = updated
	if err := j.saveLocked(); err != nil {
		j.items[id] = current
		return Record{}, err
	}
	return cloneRecord(updated), nil
}

func (j *Journal) saveLocked() error {
	return storage.SaveJSON(j.path, "transfer", "history", struct {
		Transfers map[string]Record `json:"transfers"`
	}{Transfers: cloneRecords(j.items)})
}

func normalizeRecord(record Record) Record {
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

func cloneRecords(items map[string]Record) map[string]Record {
	out := make(map[string]Record, len(items))
	for id, record := range items {
		out[id] = cloneRecord(record)
	}
	return out
}

func cloneRecord(record Record) Record {
	if record.FinishedAt != nil {
		record.FinishedAt = new(*record.FinishedAt)
	}
	return record
}

func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
