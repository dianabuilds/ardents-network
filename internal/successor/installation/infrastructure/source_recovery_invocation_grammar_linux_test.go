package infrastructure

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// All finite invocation names/schemas are exercised using detached canonical
// byte vectors. No archive, ACK or codec value supplies native/domain join.
func TestSourceRecoveryEveryInvocationRetainsCompleteOnlyHistories(t *testing.T) {
	start, retirement := sourceStartTranscriptVector(t), sourceRetirementTranscriptVector(t)
	for n := uint32(0); n < 64; n++ {
		directory, journal := "/installed/source-startup", "/installed.source-first-start-journal"
		started, retired := bytes.Clone(start), bytes.Clone(retirement)
		if n != 0 {
			directory = fmt.Sprintf("/installed/source-invocation-%08d", n)
			journal = fmt.Sprintf("/installed.source-invocation-%08d-journal", n)
			started = bytes.ReplaceAll(started, []byte("/installed.source-first-start-journal"), []byte(journal))
			started = bytes.ReplaceAll(started, []byte("/installed/source-startup"), []byte(directory))
			started = bytes.ReplaceAll(started, []byte("ardents-source-first-start-record-v1"), []byte("ardents-source-invocation-record-v1"))
			started = bytes.ReplaceAll(started, []byte(`"phase":"first-start"`), []byte(`"phase":"invocation"`))
			retired = bytes.ReplaceAll(retired, []byte("/installed/source-startup"), []byte(directory))
		}
		for _, row := range []struct {
			raw    []byte
			count  int
			decode func([]byte) error
		}{
			{started, 22, func(raw []byte) error {
				_, err := decodeSourceStartRecords(raw, sourceAttemptNumber(n), journal, directory, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), 321)
				return err
			}},
			{retired, 12, func(raw []byte) error {
				_, err := decodeSourceRetirementRecords(raw, directory, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), 321)
				return err
			}},
		} {
			if err := row.decode(row.raw); err != nil {
				t.Fatal("complete independent invocation vector refused", n, row.count, err)
			}
			lines := bytes.Split(row.raw[:len(row.raw)-1], []byte{'\n'})
			for cut := 1; cut < row.count; cut++ {
				prefix := append(bytes.Join(lines[:cut], []byte{'\n'}), '\n')
				if err := row.decode(prefix); err == nil {
					t.Fatal("partial archive/ACK/retirement became completion", n, row.count, cut)
				}
			}
		}
	}
}
