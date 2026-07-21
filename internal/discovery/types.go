package discovery

import (
	discoveryrecord "ardents/internal/discovery/records"
	"sync"
)

const LocalRecordTTL = discoveryrecord.LocalRecordTTL

type Record = discoveryrecord.Record

type Entry = discoveryrecord.Entry

type ImportResult = discoveryrecord.ImportResult

type Service struct {
	mu      sync.Mutex
	path    string
	state   string
	reason  string
	records []Entry
}

func New(path string) *Service {
	return &Service{
		path:  path,
		state: "new",
	}
}

func NewInDir(dir string) *Service {
	return New(PathInDir(dir))
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return SaveSnapshot(s.path, Snapshot{
		Records: CloneEntries(s.records),
		State:   s.state,
		Reason:  s.reason,
	})
}

func Canonical(record Record) ([]byte, error) {
	return discoveryrecord.Canonical(record)
}

func TrustStateForResult(result TrustResult) string {
	if result.Valid {
		return "ready"
	}
	if result.Outcome == "" && result.Reason == "" {
		return "new"
	}
	return "degraded"
}
