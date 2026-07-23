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
	trust   *TrustEvaluator
	persist func(string, any) error
}

func New(path string) *Service {
	return NewWithTrust(path, NewTrustEvaluator(nil))
}

func NewWithTrust(path string, trust *TrustEvaluator) *Service {
	if trust == nil {
		trust = NewTrustEvaluator(nil)
	}
	return &Service{
		path: path, state: "new", trust: trust, persist: SaveSnapshot,
	}
}

func NewInDir(dir string) *Service {
	return New(PathInDir(dir))
}

func NewInDirWithTrust(dir string, trust *TrustEvaluator) *Service {
	return NewWithTrust(PathInDir(dir), trust)
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return s.persist(s.path, Snapshot{
		SchemaVersion: 2,
		Records:       CloneEntries(s.records),
		State:         s.state,
		Reason:        s.reason,
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
