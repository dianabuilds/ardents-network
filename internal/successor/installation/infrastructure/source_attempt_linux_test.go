package infrastructure

import "testing"

func TestSourceAttemptDiscoveryDoesNotReuseGappedOrNoncanonicalHistory(t *testing.T) {
	for _, test := range []struct {
		names []string
		last  sourceAttemptNumber
	}{
		{[]string{"source-startup"}, 0},
		{[]string{"source-invocation-00000001", "source-startup"}, 1},
		{[]string{"source-invocation-00000002", "source-startup", "source-invocation-00000001"}, 2},
	} {
		last, err := sourceAttemptDirectories(test.names)
		if err != nil || last != test.last {
			t.Fatal("contiguous original directory names refused", test.names, last, err)
		}
	}
	for _, names := range [][]string{
		nil,
		{"source-invocation-00000001"},
		{"source-startup", "source-invocation-00000002"},
		{"source-startup", "source-invocation-1"},
		{"source-startup", "source-invocation-00000000"},
		{"source-startup", "source-invocation-00000064"},
		{"source-startup", "source-invocation-00000001-journal"},
		{"source-startup", "source-invocation-00000001", "source-invocation-00000001"},
		{"source-startup", "source-startup"},
	} {
		if _, err := sourceAttemptDirectories(names); err == nil {
			t.Fatal("foreign/gapped/duplicate attempt history selected", names)
		}
	}
}
