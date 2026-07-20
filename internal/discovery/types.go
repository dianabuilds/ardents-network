package discovery

import (
	"sync"

	discoveryintake "ardents/internal/discovery/intake"
	discoveryrecord "ardents/internal/discovery/record"
	statepkg "ardents/internal/discovery/state"
)

const LocalRecordTTL = discoveryrecord.LocalRecordTTL

type Record = discoveryrecord.Record

type Entry = discoveryrecord.Entry

type ImportResult = discoveryintake.ImportResult

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
	return New(statepkg.PathInDir(dir))
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return statepkg.SaveSnapshot(s.path, statepkg.Snapshot{
		Records: statepkg.CloneEntries(s.records),
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
