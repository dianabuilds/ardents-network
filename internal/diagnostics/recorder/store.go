package recorder

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	db "ardents/internal/persistence"
)

type Ledger struct {
	Seq            int64              `json:"seq,omitempty"`
	Health         health.Summary     `json:"health,omitempty"`
	RetainedHealth health.Summary     `json:"retained_health,omitempty"`
	Events         []event.Record     `json:"events,omitempty"`
	Operations     []operation.Record `json:"operations"`
}

const maxClosedOperations = 32

func Load(path string) (Ledger, error) {
	if path == "" {
		return Ledger{}, nil
	}
	raw, found, err := db.ReadProtectedFile(path)
	if err != nil {
		return Ledger{}, err
	}
	if !found {
		return Ledger{}, nil
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return Ledger{}, nil
	}
	return Decode(raw)
}

func Decode(raw []byte) (Ledger, error) {
	var stored Ledger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return Ledger{}, &CorruptLedgerError{Err: fmt.Errorf("diagnostics ledger decode failed: %w", err), Fatal: true}
	}
	return stored, nil
}

func Save(path string, ledger Ledger) error {
	if path == "" {
		return nil
	}
	items := operation.Compact(operation.Clone(ledger.Operations), maxClosedOperations)
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
	raw, err := json.MarshalIndent(Ledger{
		Seq:            ledger.Seq,
		Health:         health.CloneSummary(ledger.Health),
		RetainedHealth: health.CloneSummary(ledger.RetainedHealth),
		Events:         event.Clone(ledger.Events),
		Operations:     items,
	}, "", "  ")
	if err != nil {
		return err
	}
	return db.AtomicWritePrivateFile(path, raw)
}

func isCorruptLedger(err error) (*CorruptLedgerError, bool) {
	var target *CorruptLedgerError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
