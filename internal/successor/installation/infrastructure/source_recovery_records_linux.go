package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"syscall"
)

// sourceRecoveryCompletion is a detached codec record, not an authorization or
// proof of cleanup. Its native reader must independently retain both files,
// exclude the original writer and recovery writer, check their native custody
// and consume the full initial/replacement grammar before accepting completion.
// The original journal remains immutable; only subsequent records live here.
type sourceRecoveryCompletion struct {
	Schema           string                `json:"schema"`
	Operation        string                `json:"operation"`
	Journal          string                `json:"journal"`
	Device           uint64                `json:"device"`
	Inode            uint64                `json:"inode"`
	PrefixSHA256     string                `json:"prefix_sha256"`
	PrefixRecords    int                   `json:"prefix_records"`
	Generation       string                `json:"generation"`
	BindingSHA256    string                `json:"binding_sha256"`
	Records          sourceRecoveryRecords `json:"records"`
	TranscriptSHA256 string                `json:"transcript_sha256"`
}

// Bound record allocation before decoding an attacker-controlled JSON array;
// the byte limit alone would still admit hundreds of thousands of tiny values.
type sourceRecoveryRecords []sourceRecord

func (records *sourceRecoveryRecords) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return ErrBinding
	}
	decoded := make(sourceRecoveryRecords, 0)
	for decoder.More() {
		if len(decoded) == sourceRecoverableInitialRecords {
			return ErrBinding
		}
		var line json.RawMessage
		if decoder.Decode(&line) != nil || len(line)+1 > 32<<10 {
			return ErrBinding
		}
		var record sourceRecord
		if json.Unmarshal(line, &record) != nil {
			return ErrBinding
		}
		decoded = append(decoded, record)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') || decoder.InputOffset() != int64(len(raw)) {
		return ErrBinding
	}
	*records = decoded
	return nil
}

// decodeSourceRecoveryCompletion checks only the closed completion envelope and
// canonical transcript framing. Callers must still validate every semantic
// record through decodeSourceInitialRecords/decodeSourceReplacementRecords and
// compare its original native resources. Parseable bytes never grant a lease,
// Release decision, account provenance, completed domain join or Start right.
func decodeSourceRecoveryCompletion(raw, prefix []byte, original os.FileInfo, journal, operation, generation, binding string) ([]byte, error) {
	count := 0
	switch operation {
	case "initial":
		count = sourceRecoverableInitialRecords
	case "replacement":
		count = 55
	default:
		return nil, ErrBinding
	}
	if !sourcePath(journal) || !sourceInitialSHA(generation) || !sourceInitialSHA(binding) ||
		!sourceAccess(original, 0600, 0, 0, false) || original.Size() != int64(len(prefix)) ||
		len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' {
		return nil, ErrBinding
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || native.Dev == 0 || native.Ino == 0 {
		return nil, ErrBinding
	}
	before, err := decodeSourceRecoveryPrefix(prefix, count)
	if err != nil || len(before) == 0 || before[0].Path != journal ||
		before[0].Device != uint64(native.Dev) || before[0].Inode != native.Ino ||
		before[0].SHA256 != "" || before[0].Mode != 0600 || before[0].UID != 0 || before[0].GID != 0 {
		return nil, ErrBinding
	}
	count = sourceRecoveryCompleteCount(operation, before[0].Phase)
	if count == 0 || len(before) > count {
		return nil, ErrBinding
	}
	var completion sourceRecoveryCompletion
	if json.Unmarshal(raw, &completion) != nil {
		return nil, ErrBinding
	}
	canonical, err := json.Marshal(completion)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) ||
		completion.Schema != "ardents-source-recovery-completion-v1" || completion.Operation != operation ||
		completion.Journal != journal || completion.Device != uint64(native.Dev) || completion.Inode != native.Ino ||
		completion.PrefixSHA256 != digest(prefix) || completion.PrefixRecords != len(before) ||
		completion.Generation != generation || completion.BindingSHA256 != binding ||
		completion.Records == nil || len(before)+len(completion.Records) != count {
		return nil, ErrBinding
	}
	transcript := bytes.Clone(prefix)
	for i, record := range completion.Records {
		if record.Sequence != len(before)+i {
			return nil, ErrBinding
		}
		line, err := json.Marshal(record)
		if err != nil || len(line)+1 > 32<<10 || len(transcript)+len(line)+1 > 4<<20 {
			return nil, ErrBinding
		}
		transcript = append(transcript, line...)
		transcript = append(transcript, '\n')
	}
	if !sourceInitialSHA(completion.TranscriptSHA256) || digest(transcript) != completion.TranscriptSHA256 {
		return nil, ErrBinding
	}
	return transcript, nil
}

// This canonical prefix parser deliberately accepts no incomplete last record,
// missing sequence, unknown field or alternative JSON spelling. It does not
// recognize a phase as permission; the operation's full grammar owns ordering.
func decodeSourceRecoveryPrefix(raw []byte, maximum int) ([]sourceRecord, error) {
	if maximum != 55 && maximum != 112 && maximum != sourceRecoverableInitialRecords {
		return nil, ErrBinding
	}
	return decodeSourceCanonicalPrefix(raw, maximum)
}

// Shared framing only. Each caller supplies its own closed record count and
// semantic grammar; framing cannot extend an operation's recovery contract.
func decodeSourceCanonicalPrefix(raw []byte, maximum int) ([]sourceRecord, error) {
	if maximum < 1 || maximum > sourceRecoverableInitialRecords || len(raw) == 0 || len(raw) > 4<<20 || raw[len(raw)-1] != '\n' {
		return nil, ErrBinding
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) == 0 || len(lines) > maximum {
		return nil, ErrBinding
	}
	records := make([]sourceRecord, len(lines))
	for i, line := range lines {
		if len(line)+1 > 32<<10 || json.Unmarshal(line, &records[i]) != nil || records[i].Sequence != i {
			return nil, ErrBinding
		}
		canonical, err := json.Marshal(records[i])
		if err != nil || !bytes.Equal(line, canonical) {
			return nil, ErrBinding
		}
	}
	return records, nil
}

// A completion envelope cannot widen either existing operation grammar. This
// validates detached complete records only; its native consumer must still
// compare every recorded resource, retained lineage and fresh authority.
func decodeSourceRecoveredRecords(transcript, binding []byte, operation string, hashes map[string]string, admitted, previousUnit string) (map[string]sourceRecord, error) {
	configuration, replacement, err := decodeSourceSelectedBinding(binding)
	if err != nil {
		return nil, err
	}
	if replacement == nil {
		if operation != "initial" || previousUnit != "" {
			return nil, ErrBinding
		}
		return decodeSourceInitialBirthRecords(transcript, configuration, hashes, admitted)
	}
	if operation != "replacement" {
		return nil, ErrBinding
	}
	return decodeSourceReplacementRecords(transcript, *replacement, hashes, admitted, previousUnit)
}

func sourceRecoveryCompleteCount(operation, firstPhase string) int {
	if operation == "initial" {
		if firstPhase == "initial" {
			return 112
		}
		if firstPhase == sourceRecoverableInitialPhase {
			return sourceRecoverableInitialRecords
		}
	}
	if operation == "replacement" && firstPhase == "initial" {
		return 55
	}
	return 0
}
