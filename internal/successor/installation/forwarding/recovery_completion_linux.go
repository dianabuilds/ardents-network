package forwarding

import (
	"bytes"
	"encoding/json"
)

// A separate stopped completion always names its immutable original prefix and
// genuine cut. This envelope is not an original writer result or a live permit.
type recoveryCompletion struct {
	Schema              string            `json:"schema"`
	Operation           string            `json:"operation"`
	Journal             string            `json:"journal"`
	Device              uint64            `json:"device"`
	Inode               uint64            `json:"inode"`
	PrefixSHA256        string            `json:"prefix_sha256"`
	PrefixRecords       uint32            `json:"prefix_records"`
	Generation          string            `json:"generation"`
	BindingSHA256       string            `json:"binding_sha256"`
	OriginalCutSHA256   string            `json:"original_cut_sha256"`
	Outcome             string            `json:"outcome"`
	Records             []json.RawMessage `json:"records"`
	TranscriptSHA256    string            `json:"transcript_sha256"`
	NativeContextSHA256 string            `json:"native_context_sha256,omitempty"`
}

func recoveryPaths(journal string) (control, completion, pending string) {
	return journal + ".recovery-journal", journal + ".recovery-completion", journal + ".recovery-pending"
}

func decodeRecoveryCompletion(raw, prefix, cutRaw []byte, cut transitionCutResult) (recoveryCompletion, []byte, error) {
	var result recoveryCompletion
	if len(raw) == 0 || len(raw) > 4<<20 || json.Unmarshal(raw, &result) != nil {
		return result, nil, ErrBinding
	}
	canonical, err := json.Marshal(result)
	b := cut.Boundary
	reload := result.Operation == "initial" && initialPreReloadCut(b.Records) || result.Operation == "replacement" && (b.Records == 57 || b.Records == 58)
	version := result.Schema == "ardents-forwarding-recovery-completion-v1" && result.NativeContextSHA256 == "" || result.Schema == "ardents-forwarding-recovery-completion-v2" && reload && validStartDigest(result.NativeContextSHA256)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || !version || result.Operation != b.Operation || result.Journal != b.Journal || result.Device != b.Device || result.Inode != b.Inode || result.PrefixSHA256 != digest(prefix) || result.PrefixSHA256 != b.PrefixSHA256 || result.PrefixRecords != b.Records || result.Generation != b.Generation || result.BindingSHA256 != b.BindingSHA256 || result.OriginalCutSHA256 != digest(cutRaw) || result.Records == nil || !validStartDigest(result.TranscriptSHA256) {
		return result, nil, ErrBinding
	}
	original, err := decodeTransitionPrefix(prefix, b.Operation)
	if err != nil || uint32(len(original)) != b.Records || len(result.Records) > int(transitionRecordLimit(b.Operation))-len(original) {
		return result, nil, ErrBinding
	}
	full := bytes.Clone(prefix)
	for n, rawRecord := range result.Records {
		if len(rawRecord) == 0 || len(rawRecord)+1 > 32<<10 || bytes.IndexByte(rawRecord, '\n') >= 0 {
			return result, nil, ErrBinding
		}
		var record initialRecord
		var canonical []byte
		if b.Operation == "initial" || b.Operation == "replacement" {
			if json.Unmarshal(rawRecord, &record) != nil {
				return result, nil, ErrBinding
			}
			canonical, _ = json.Marshal(record)
		} else {
			var entry startEntry
			if json.Unmarshal(rawRecord, &entry) != nil {
				return result, nil, ErrBinding
			}
			expected := "ardents-forwarding-start-journal-v1"
			if b.Operation == "retirement" {
				expected = "ardents-forwarding-retirement-journal-v1"
			}
			if entry.Schema != expected || entry.Sequence >= transitionRecordLimit(b.Operation) {
				return result, nil, ErrBinding
			}
			canonical, _ = json.Marshal(entry)
			record = initialRecord{int(entry.Sequence), entry.Phase, entry.Path, entry.Device, entry.Inode, entry.SHA256, entry.Mode, entry.UID, entry.GID}
		}
		if !bytes.Equal(rawRecord, canonical) || record.Sequence != len(original)+n {
			return result, nil, ErrBinding
		}
		if b.Operation == "start" {
			switch record.Phase {
			case "terminal-directory-intent", "terminal-directory-born", "terminal-directory-ready", "stopped-before-start":
			default:
				return result, nil, ErrBinding
			}
		} else if b.Operation == "retirement" && record.Phase != "retired" {
			// Original domain/native joins, result creation and archival cannot
			// be supplied by the later recovery producer.
			return result, nil, ErrBinding
		}
		full = append(full, canonical...)
		full = append(full, '\n')
	}
	if len(full) > 4<<20 || digest(full) != result.TranscriptSHA256 {
		return result, nil, ErrBinding
	}
	rows, err := decodeTransitionPrefix(full, b.Operation)
	if err != nil {
		return result, nil, err
	}
	switch b.Operation {
	case "initial", "replacement":
		complete := uint32(len(rows)) == transitionRecordLimit(b.Operation)
		if b.Operation == "replacement" && len(rows) == 56 && rows[0].Phase == "forwarding-replacement-v1" {
			complete = true
		}
		if result.Outcome != "selected-stopped" || !complete {
			return result, nil, ErrBinding
		}
	case "retirement":
		if result.Outcome != "retired-stopped" || len(rows) != 18 || rows[len(rows)-1].Phase != "retired" {
			return result, nil, ErrBinding
		}
	case "start":
		if result.Outcome != "stopped-before-start" || len(result.Records) == 0 || rows[len(rows)-1].Phase != "stopped-before-start" {
			return result, nil, ErrBinding
		}
	default:
		return result, nil, ErrBinding
	}
	// Native identities, full operation grammar, actual safe classification,
	// fresh authority and all original/recovery guards remain separate checks.
	return result, full, nil
}
