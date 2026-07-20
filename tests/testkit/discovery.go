package testkit

import (
	"fmt"
	"testing"
	"time"

	discoveryapi "ardents/internal/discovery/api"
	nodeapi "ardents/internal/node/api"

	"github.com/stretchr/testify/require"
)

type discoveryRecordsReader interface {
	ListRecords() ([]discoveryapi.DiscoveryRecord, error)
}

type discoveryImportTarget interface {
	ImportRecord(discoveryapi.DiscoveryRecord) (discoveryapi.RecordImportResult, error)
}

type serviceResolver interface {
	ResolveService(string) (discoveryapi.ServiceResult, error)
}

type snapshotReader interface {
	Snapshot() nodeapi.Snapshot
}

func BootstrapEndpoints(t *testing.T, n discoveryRecordsReader) []string {
	t.Helper()

	records, err := n.ListRecords()
	require.NoError(t, err)

	for _, record := range records {
		if record.Kind == "node" && len(record.Endpoints) > 0 {
			return append([]string(nil), record.Endpoints...)
		}
	}

	require.FailNow(t, "expected node bootstrap endpoints")
	return nil
}

func ImportRecordsFromNode(t *testing.T, target discoveryImportTarget, source discoveryRecordsReader, sourceLabel string, filter func(discoveryapi.DiscoveryRecord) bool) []discoveryapi.DiscoveryRecord {
	t.Helper()

	records, err := source.ListRecords()
	require.NoError(t, err)

	imported := make([]discoveryapi.DiscoveryRecord, 0, len(records))
	for _, record := range records {
		if filter != nil && !filter(record) {
			continue
		}
		record.Source = sourceLabel
		_, err := target.ImportRecord(record)
		require.NoError(t, err)
		imported = append(imported, record)
	}

	return imported
}

func WaitForServiceMatchCount(t *testing.T, timeout time.Duration, n serviceResolver, service string, want int) discoveryapi.ServiceResult {
	t.Helper()

	var last discoveryapi.ServiceResult
	WaitForCondition(t, timeout, fmt.Sprintf("service %s match count %d", service, want), func() (bool, string) {
		result, err := n.ResolveService(service)
		if err != nil {
			return false, err.Error()
		}
		last = result
		return len(result.Matches) == want, fmt.Sprintf("outcome=%q matches=%d", result.Outcome, len(result.Matches))
	})

	return last
}

func WaitForSnapshot(t *testing.T, timeout time.Duration, n snapshotReader, description string, check func(nodeapi.Snapshot) (bool, string)) nodeapi.Snapshot {
	t.Helper()

	var last nodeapi.Snapshot
	WaitForCondition(t, timeout, description, func() (bool, string) {
		last = n.Snapshot()
		return check(last)
	})

	return last
}
