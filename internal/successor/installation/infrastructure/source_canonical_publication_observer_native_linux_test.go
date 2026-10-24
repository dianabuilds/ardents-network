//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type sourcePhysicalPublicationPreimage struct {
	record    sourceRecord
	count     int
	prefixSHA string
}

func (p *sourcePhysicalCut) needsPublicationPreimage() bool {
	return p.canonical != nil && len(p.canonical.Arguments) >= 2 && p.canonical.Arguments[1] == "replace-source" &&
		(p.window == "binding-before-record" || p.window == "unit-before-record")
}

// Called only at the actual original intent entry, before its journal append
// and before fixedfile.Commit can change the old image. A later observation of
// new bytes cannot manufacture this original preimage.
func (p *sourcePhysicalCut) capturePublicationPreimage(records []sourceRecord, path, prefixSHA string) error {
	if !p.needsPublicationPreimage() || p.publicationBefore != nil || !sourceCutObserverSHA(prefixSHA) {
		return errors.New("one original publication preimage observation required")
	}
	phase := "predecessor-binding"
	if p.window == "unit-before-record" {
		phase = "predecessor-unit"
	}
	var before sourceRecord
	matches := 0
	for _, record := range records {
		if record.Phase == phase && record.Path == path {
			before, matches = record, matches+1
		}
	}
	if matches != 1 || before.Mode != 0644 || before.UID != 0 || before.GID != 0 || !sourceCutObserverSHA(before.SHA256) {
		return errors.New("one exact recorded original publication preimage required")
	}
	body, _, err := readSourceRecoveryHeldPayload(path, before)
	if err != nil || sourcePhysicalSHA(body) != before.SHA256 {
		return errors.Join(errors.New("actual original publication preimage bytes/inode differ"), err)
	}
	p.publicationBefore = &sourcePhysicalPublicationPreimage{record: before, count: len(records), prefixSHA: prefixSHA}
	return nil
}

func sourcePhysicalPrefixSHA(records []sourceRecord) (string, error) {
	var raw []byte
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		raw = append(raw, encoded...)
		raw = append(raw, '\n')
	}
	return sourcePhysicalSHA(raw), nil
}

func (p *sourcePhysicalCut) canonicalPublication(records []sourceRecord, root, target string) ([]byte, os.FileInfo, error) {
	inventory, _, err := p.canonicalSealedInventory(records, root, false)
	if err != nil {
		return nil, nil, err
	}
	name := "binding.json"
	if p.window == "unit-before-record" {
		name = "rendered-source.service"
	}
	candidate := filepath.Join(root, "generations", p.canonical.Generation, name)
	var candidateSHA string
	for _, record := range records {
		if record.Phase == "file-ready" && record.Path == candidate {
			candidateSHA = record.SHA256
		}
	}
	last := records[len(records)-1]
	if !sourceCutObserverSHA(candidateSHA) || last.Path != target || last.SHA256 != candidateSHA || last.Mode != 0644 || last.UID != 0 || last.GID != 0 {
		return nil, nil, errors.New("original fixed publication must match sealed candidate bytes/access")
	}
	if p.needsPublicationPreimage() {
		before := p.publicationBefore
		if before == nil || before.count != len(records)-1 || last.Device != before.record.Device || last.Inode != before.record.Inode || last.Phase != p.lastPhase {
			return nil, nil, errors.New("original pre-Commit publication observation/inode missing")
		}
		prefixSHA, err := sourcePhysicalPrefixSHA(records[:before.count])
		if err != nil || prefixSHA != before.prefixSHA {
			return nil, nil, errors.Join(errors.New("original pre-publication prefix changed"), err)
		}
	} else {
		if last.Phase != "file-ready" {
			return nil, nil, errors.New("original initial fixed-file ready required")
		}
		if err := sourcePhysicalBirthTriple(records, last, "file"); err != nil {
			return nil, nil, err
		}
	}
	body, info, err := readSourceRecoveryHeldPayload(target, last)
	if err != nil || sourcePhysicalSHA(body) != candidateSHA {
		return nil, nil, errors.Join(errors.New("actual original published fixed-file bytes/inode required"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	actual, err := json.Marshal(sourcePhysicalSealedFile{Record: last, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
	if err != nil {
		return nil, nil, err
	}
	return bytes.Join([][]byte{inventory, actual}, []byte{'\n'}), info, nil
}
