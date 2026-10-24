package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Codec fixtures deliberately supply detached metadata and syntactic records.
// They construct no Installation, Release, State, manager or native admission.
type sourceRecoveryCodecInfo struct {
	size int64
	mode os.FileMode
	stat syscall.Stat_t
}

func (f sourceRecoveryCodecInfo) Name() string       { return "journal" }
func (f sourceRecoveryCodecInfo) Size() int64        { return f.size }
func (f sourceRecoveryCodecInfo) Mode() os.FileMode  { return f.mode }
func (f sourceRecoveryCodecInfo) ModTime() time.Time { return time.Time{} }
func (f sourceRecoveryCodecInfo) IsDir() bool        { return f.mode.IsDir() }
func (f sourceRecoveryCodecInfo) Sys() any           { return &f.stat }

func sourceRecoveryCodecFixture(t *testing.T, operation string, cut int) (sourceRecoveryCompletion, []byte, []byte, sourceRecoveryCodecInfo) {
	t.Helper()
	count := 112
	if operation == "replacement" {
		count = 55
	}
	all := make(sourceRecoveryRecords, count)
	var transcript, prefix []byte
	for i := range all {
		all[i] = sourceRecord{Sequence: i, Phase: "codec-only", Path: "/var/lib/not-an-installation"}
		if i == 0 {
			all[i] = sourceRecord{Phase: "initial", Path: "/var/lib/original-journal", Device: 41, Inode: 71, Mode: 0600}
		}
		line, err := json.Marshal(all[i])
		if err != nil {
			t.Fatal(err)
		}
		transcript = append(transcript, line...)
		transcript = append(transcript, '\n')
		if i+1 == cut {
			prefix = bytes.Clone(transcript)
		}
	}
	completion := sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: operation,
		Journal: all[0].Path, Device: 41, Inode: 71, PrefixSHA256: digest(prefix), PrefixRecords: cut,
		Generation: strings.Repeat("a", 64), BindingSHA256: strings.Repeat("b", 64),
		Records: append(make(sourceRecoveryRecords, 0), all[cut:]...), TranscriptSHA256: digest(transcript)}
	info := sourceRecoveryCodecInfo{size: int64(len(prefix)), mode: 0600, stat: syscall.Stat_t{Dev: 41, Ino: 71, Nlink: 1}}
	return completion, prefix, transcript, info
}

func marshalSourceRecoveryCodec(t *testing.T, completion sourceRecoveryCompletion) []byte {
	t.Helper()
	raw, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestSourceRecoveryCompletionPreservesEveryOriginalPrefix(t *testing.T) {
	for _, operation := range []string{"initial", "replacement"} {
		count := 112
		if operation == "replacement" {
			count = 55
		}
		for cut := 1; cut <= count; cut++ {
			completion, prefix, expected, info := sourceRecoveryCodecFixture(t, operation, cut)
			original := bytes.Clone(prefix)
			actual, err := decodeSourceRecoveryCompletion(marshalSourceRecoveryCodec(t, completion), prefix, info,
				completion.Journal, operation, completion.Generation, completion.BindingSHA256)
			if err != nil || !bytes.Equal(actual, expected) || !bytes.Equal(prefix, original) {
				t.Fatalf("%s prefix%d: original prefix/completed bytes differ: %v", operation, cut, err)
			}
			// Returned bytes are detached; a caller cannot mutate the retained
			// original prefix through the codec result.
			actual[0] ^= 1
			if !bytes.Equal(prefix, original) {
				t.Fatal("completion aliases original journal bytes")
			}
		}
	}
}

func TestSourceRecoveryCompletionRejectsForeignOrIncompleteEnvelope(t *testing.T) {
	for _, change := range []struct {
		name string
		edit func(*sourceRecoveryCompletion)
	}{
		{"schema", func(c *sourceRecoveryCompletion) { c.Schema += "-unknown" }},
		{"start-operation", func(c *sourceRecoveryCompletion) { c.Operation = "start" }},
		{"journal", func(c *sourceRecoveryCompletion) { c.Journal += "-copied" }},
		{"device", func(c *sourceRecoveryCompletion) { c.Device++ }},
		{"inode", func(c *sourceRecoveryCompletion) { c.Inode++ }},
		{"prefix-hash", func(c *sourceRecoveryCompletion) { c.PrefixSHA256 = strings.Repeat("c", 64) }},
		{"prefix-count", func(c *sourceRecoveryCompletion) { c.PrefixRecords++ }},
		{"generation", func(c *sourceRecoveryCompletion) { c.Generation = strings.Repeat("d", 64) }},
		{"binding", func(c *sourceRecoveryCompletion) { c.BindingSHA256 = strings.Repeat("e", 64) }},
		{"missing-suffix", func(c *sourceRecoveryCompletion) { c.Records = c.Records[:len(c.Records)-1] }},
		{"sequence", func(c *sourceRecoveryCompletion) { c.Records[0].Sequence++ }},
		{"transcript", func(c *sourceRecoveryCompletion) { c.TranscriptSHA256 = strings.Repeat("f", 64) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			completion, prefix, _, info := sourceRecoveryCodecFixture(t, "replacement", 14)
			journal, generation, binding := completion.Journal, completion.Generation, completion.BindingSHA256
			change.edit(&completion)
			if _, err := decodeSourceRecoveryCompletion(marshalSourceRecoveryCodec(t, completion), prefix, info, journal, "replacement", generation, binding); err == nil {
				t.Fatal("foreign/incomplete completion accepted")
			}
		})
	}
}

func TestSourceRecoveryCompletionRejectsUnknownSpellingAndNativeSubstitution(t *testing.T) {
	completion, prefix, _, info := sourceRecoveryCodecFixture(t, "initial", 67)
	raw := marshalSourceRecoveryCodec(t, completion)
	for _, changed := range [][]byte{
		bytes.TrimSuffix(raw, []byte{'\n'}),
		append(bytes.Clone(raw), '\n'),
		bytes.Replace(raw, []byte(`"operation":`), []byte(`"unknown":true,"operation":`), 1),
		bytes.Replace(raw, []byte(`"operation":`), []byte(`"operation":"initial","operation":`), 1),
		bytes.Replace(raw, []byte(`"device":41`), []byte(`"device": 41`), 1),
	} {
		if _, err := decodeSourceRecoveryCompletion(changed, prefix, info, completion.Journal, completion.Operation, completion.Generation, completion.BindingSHA256); err == nil {
			t.Fatal("noncanonical completion accepted")
		}
	}
	for _, changed := range []sourceRecoveryCodecInfo{
		{size: info.size - 1, mode: info.mode, stat: info.stat},
		{size: info.size, mode: 0644, stat: info.stat},
		{size: info.size, mode: info.mode, stat: syscall.Stat_t{Dev: 42, Ino: 71, Nlink: 1}},
		{size: info.size, mode: info.mode, stat: syscall.Stat_t{Dev: 41, Ino: 72, Nlink: 1}},
		{size: info.size, mode: info.mode, stat: syscall.Stat_t{Dev: 41, Ino: 71, Nlink: 2}},
		{size: info.size, mode: info.mode, stat: syscall.Stat_t{Dev: 41, Ino: 71, Nlink: 1, Uid: 100}},
	} {
		if _, err := decodeSourceRecoveryCompletion(raw, prefix, changed, completion.Journal, completion.Operation, completion.Generation, completion.BindingSHA256); err == nil {
			t.Fatal("substituted original journal accepted")
		}
	}
	if _, err := decodeSourceRecoveryCompletion(raw, prefix, info, completion.Journal, "retirement", completion.Generation, completion.BindingSHA256); err == nil {
		t.Fatal("retirement treated as continuable transcript")
	}
}

func TestSourceRecoveryPrefixRejectsTornUnknownAndUnboundedRecords(t *testing.T) {
	_, prefix, _, _ := sourceRecoveryCodecFixture(t, "replacement", 14)
	for _, changed := range [][]byte{
		nil,
		prefix[:len(prefix)-1],
		append(bytes.Clone(prefix), '\n'),
		bytes.Replace(prefix, []byte(`"sequence":1,`), []byte(`"sequence":0,`), 1),
		bytes.Replace(prefix, []byte(`"phase":`), []byte(`"extra":0,"phase":`), 1),
		bytes.Repeat(prefix, 5),
	} {
		if _, err := decodeSourceRecoveryPrefix(changed, 55); err == nil {
			t.Fatal("torn/unknown/oversized prefix accepted")
		}
	}
	var records sourceRecoveryRecords
	for _, raw := range []string{"null", `{}`, `[` + strings.Repeat(`{},`, 115) + `{}]`, `[{"path":"` + strings.Repeat("x", 32<<10) + `"}]`} {
		if json.Unmarshal([]byte(raw), &records) == nil {
			t.Fatal("unbounded or invalid recovery record array accepted")
		}
	}
}
