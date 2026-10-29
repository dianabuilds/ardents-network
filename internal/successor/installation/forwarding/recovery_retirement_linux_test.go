package forwarding

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

// Detached byte vectors, not genuine cut/process/domain/Release authority.
func retirementRecoveryVector(t *testing.T) ([]retirementEntry, generationSelection, string, startArchive, [16]byte, []byte, transitionCutResult) {
	t.Helper()
	selected, err := decodeSelection(forwardingBindingVector())
	if err != nil {
		t.Fatal(err)
	}
	b := selected.binding
	id, challenge := [16]byte{9}, [32]byte{7}
	startSHA := digest([]byte("detached original start archive fixture"))
	archive := startArchive{Schema: "ardents-forwarding-start-archive-v1", Generation: b.Generation, BindingSHA256: digest(selected.raw), PID: 7317, InvocationID: hex.EncodeToString(id[:]), Manager: "physical-test-only", StartedRealtime: 30, StartedMonotonic: 40}
	start, err := completion.Encode(id, b.Generation, digest(selected.raw))
	if err != nil {
		t.Fatal(err)
	}
	frame := retirementFrame(start, challenge)
	result := retirementResult{Schema: "ardents-forwarding-retirement-v1", Generation: b.Generation, BindingSHA256: digest(selected.raw), StartSHA256: startSHA, InvocationID: archive.InvocationID, PID: archive.PID, Manager: archive.Manager, Challenge: hex.EncodeToString(challenge[:]), CompletionSHA256: digest(frame[:]), StartedRealtime: 30, StartedMonotonic: 40, ExitedRealtime: 50, ExitedMonotonic: 60, Code: 1}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	fixture, _ := retirementRecordVector()
	var rows []retirementEntry
	for _, line := range bytes.Split(fixture[:len(fixture)-1], []byte{'\n'}) {
		var row retirementEntry
		if json.Unmarshal(line, &row) != nil {
			t.Fatal("independent fixture")
		}
		if row.Path == "/opt/f.forwarding-retirement-00000000-journal" {
			row.Path = attemptNumber(0).retirementJournal(b.Request.InstallationRoot)
		} else {
			row.Path = strings.Replace(row.Path, "/opt/f/forwarding-retirement-00000000", attemptNumber(0).retirement(b.Request.InstallationRoot), 1)
		}
		if row.GID == 988 {
			row.GID = b.GID
		}
		switch row.Sequence {
		case 0:
			row.SHA256 = startSHA
		case 4, 5, 6:
			row.SHA256 = digest([]byte("ardents-forwarding-retirement-pending-v1\n" + startSHA + "\n"))
		case 10:
			row.SHA256 = digest(challenge[:])
		case 11:
			row.SHA256 = result.CompletionSHA256
		case 12, 13, 14, 15, 16, 17:
			row.SHA256 = digest(raw)
		}
		rows = append(rows, row)
	}
	native := nativeContextByteVector()
	cut := transitionCutResult{Boundary: transitionMessage{Operation: "retirement", Journal: rows[0].Path, Device: rows[0].Device, Inode: rows[0].Inode, Generation: b.Generation, BindingSHA256: digest(selected.raw), Binding: selected.raw, OriginalContext: native, NativeContext: native, Manager: archive.Manager}, Origin: transitionOrigin{ProgramSHA256: b.ProgramSHA256}}
	return rows, selected, startSHA, archive, id, raw, cut
}

func encodeRetirementRecoveryVector(t *testing.T, rows []retirementEntry) []byte {
	t.Helper()
	var raw []byte
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
	}
	return raw
}

func TestForwardingRecoveryRetirementPreservesOriginalJoinAndResult(t *testing.T) {
	rows, selected, startSHA, archive, id, result, cut := retirementRecoveryVector(t)
	whole := encodeRetirementRecoveryVector(t, rows)
	for _, count := range []int{17, 18} {
		prefix := encodeRetirementRecoveryVector(t, rows[:count])
		unchanged := bytes.Clone(prefix)
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		original := &recoveryOriginal{cut: cut, prefix: &transitionPrefix{journal: &initialFile{body: prefix}}, result: &initialFile{body: []byte("detached original cut fixture\n")}}
		h := &retirementSnapshot{number: 0, path: attemptNumber(0).retirement(selected.binding.Request.InstallationRoot), startSHA: startSHA, archive: archive, id: id, files: map[string]*initialFile{}}
		h.files[filepath.Join(h.path, "retirement-result.json")] = &initialFile{body: result}
		raw, err := stoppedRetirementCompletion(original, &recoveryCandidate{selection: selected}, h)
		if err != nil {
			t.Fatal(count, err)
		}
		envelope, logical, err := decodeRecoveryCompletion(raw, prefix, original.result.body, cut)
		if err != nil || !bytes.Equal(prefix, unchanged) || !bytes.Equal(logical, whole) || len(envelope.Records) != 18-count || envelope.Outcome != "retired-stopped" {
			t.Fatal("original retirement/result changed", count, err)
		}
		for _, fault := range []string{"domain-join", "native-join", "native-close", "result-ready", "program", "binding", "manager", "context", "challenge", "exit", "result-bytes"} {
			bad := append([]retirementEntry(nil), rows[:count]...)
			badCut := cut
			rawResult := result
			switch fault {
			case "domain-join":
				bad[11].SHA256 = digest([]byte("foreign frame"))
			case "native-join":
				bad[12].SHA256 = digest([]byte("foreign result"))
			case "native-close":
				bad[13].Phase = "native-joined"
			case "result-ready":
				bad[16].Inode++
			case "program":
				badCut.Origin.ProgramSHA256 = digest([]byte("foreign program"))
			case "binding":
				badCut.Boundary.Binding = []byte("foreign binding\n")
			case "manager":
				badCut.Boundary.Manager = "foreign-manager"
			case "context":
				badCut.Boundary.NativeContext = bytes.Replace(cut.Boundary.NativeContext, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
			case "challenge":
				rawResult = bytes.Replace(result, []byte(`"challenge":"07`), []byte(`"challenge":"08`), 1)
			case "exit":
				rawResult = bytes.Replace(result, []byte(`"exited_monotonic":60`), []byte(`"exited_monotonic":39`), 1)
			case "result-bytes":
				rawResult = append(bytes.Clone(result), '\n')
			}
			if _, err := stoppedRetirementTranscript(encodeRetirementRecoveryVector(t, bad), badCut, selected, 0, startSHA, archive, id, rawResult); err == nil {
				t.Fatal("foreign original proof accepted", count, fault)
			}
		}
	}
	for count := 0; count < 17; count++ {
		if _, err := stoppedRetirementTranscript(encodeRetirementRecoveryVector(t, rows[:count]), cut, selected, 0, startSHA, archive, id, result); err == nil {
			t.Fatal("unproved original result/close accepted", count)
		}
	}
}
