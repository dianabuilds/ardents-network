package testkit

import (
	"fmt"
	"testing"
	"time"

	nodeapi "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"

	"github.com/stretchr/testify/require"
)

type discoveryRecordsReader interface {
	ListRecords() ([]discoveryapi.CatalogRecordSnapshot, error)
}

type discoveryImportTarget interface {
	ImportRecord(discoveryapi.CatalogRecordSnapshot) (discoveryapi.RecordImportResult, error)
}

type serviceResolver interface {
	ResolveService(string) (discoveryapi.ServiceResult, error)
}

type snapshotReader interface {
	Snapshot() nodeapi.SystemSnapshot
}

func BootstrapEndpoints(t *testing.T, n discoveryRecordsReader) []string {
	t.Helper()

	records, err := n.ListRecords()
	require.NoError(t, err)

	for _, record := range records {
		if record.Kind() == "node" && record.Node != nil && len(record.Node.Endpoints) > 0 {
			return append([]string(nil), record.Node.Endpoints...)
		}
	}

	require.FailNow(t, "expected node bootstrap endpoints")
	return nil
}

func ImportRecordsFromNode(t *testing.T, target discoveryImportTarget, source discoveryRecordsReader, sourceLabel string, filter func(discoveryapi.CatalogRecordSnapshot) bool) []discoveryapi.CatalogRecordSnapshot {
	t.Helper()

	records, err := source.ListRecords()
	require.NoError(t, err)

	imported := make([]discoveryapi.CatalogRecordSnapshot, 0, len(records))
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

func WaitForSnapshot(t *testing.T, timeout time.Duration, n snapshotReader, description string, check func(nodeapi.SystemSnapshot) (bool, string)) nodeapi.SystemSnapshot {
	t.Helper()

	var last nodeapi.SystemSnapshot
	WaitForCondition(t, timeout, description, func() (bool, string) {
		last = n.Snapshot()
		return check(last)
	})

	return last
}
