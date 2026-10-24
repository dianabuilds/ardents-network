//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
)

// This inventory commits actual native observations, not Source authority.
// Include each file's change/modify times so a post-hold same-inode rewrite
// cannot disappear behind restored bytes, access and directory metadata.
type sourcePhysicalSealedFile struct {
	Record   sourceRecord
	Size     int64
	Changed  syscall.Timespec
	Modified syscall.Timespec
}

func (p *sourcePhysicalCut) canonicalImage(records []sourceRecord, root string) ([]byte, os.FileInfo, error) {
	if len(records) == 0 {
		return nil, nil, errors.New("original physical prefix required")
	}
	if p.window == "account-before-complete-birth" || p.window == "directory-born-before-record" {
		return p.canonicalEarlyImage(records, root)
	}
	if p.window == "seal-before-sealed" {
		return p.canonicalSealedInventory(records, root, true)
	}
	if p.window == "binding-before-record" || p.window == "unit-before-record" {
		return p.canonicalPublication(records, root, p.canonicalTarget(root))
	}
	if p.window == "reload-before-verified" || p.window == "journal-complete-before-close" || p.window == "pending-before-unlink" {
		return p.canonicalCompletedImage(records, root, "/etc/systemd/system/ardents-source.service")
	}
	return p.canonicalPayload(records[len(records)-1], root)
}

// Retain the original directory handle throughout the whole finite inventory.
// Neither a chmod alone nor complete-looking copied files prove the observed
// original sealing effect or an eligible original-cut completion.
func (p *sourcePhysicalCut) canonicalSealedInventory(records []sourceRecord, root string, beforeSealed bool) (raw []byte, info os.FileInfo, returned error) {
	path := filepath.Join(root, "generations", p.canonical.Generation)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	info, err = f.Stat()
	if err != nil || !sourceAccess(info, 0555, 0, 0, true) {
		return nil, nil, errors.Join(errors.New("actual original sealed root0555 directory required"), err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, nil, errors.Join(errors.New("original sealed directory must be ext4"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	expected := sourceRecord{Phase: "seal-intent", Path: path,
		Device: uint64(native.Dev), Inode: native.Ino, SHA256: p.canonical.Generation, Mode: 0555}
	index, matches := -1, 0
	for i, record := range records {
		if record.Phase == "seal-intent" && record.Path == path {
			index, matches = i, matches+1
		}
	}
	expected.Sequence = index
	if matches != 1 || records[index] != expected {
		return nil, nil, errors.New("exact original seal intent/inode required")
	}
	if beforeSealed {
		if index != len(records)-1 {
			return nil, nil, errors.New("seal effect must precede original sealed record")
		}
	} else {
		sealed := expected
		sealed.Sequence, sealed.Phase = index+1, "sealed"
		if index+1 >= len(records) || records[index+1] != sealed {
			return nil, nil, errors.New("original completed seal pair required")
		}
	}
	directory := expected
	directory.Phase, directory.Mode, directory.SHA256 = "directory-ready", 0755, ""
	if err := sourcePhysicalBirthTriple(records, directory, "directory"); err != nil {
		return nil, nil, err
	}
	names := sourceGenerationNames()
	slices.Sort(names)
	if err := checkSourceInventory(path, names); err != nil {
		return nil, nil, errors.Join(errors.New("exact sealed generation inventory required"), err)
	}
	observed := make([]sourcePhysicalSealedFile, 0, len(names)+1)
	observed = append(observed, sourcePhysicalSealedFile{Record: expected, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
	for _, name := range names {
		filePath := filepath.Join(path, name)
		var ready sourceRecord
		matches := 0
		for _, record := range records {
			if record.Phase == "file-ready" && record.Path == filePath {
				ready, matches = record, matches+1
			}
		}
		mode := uint32(0444)
		if name == ProgramName {
			mode = 0555
		}
		if matches != 1 || ready.Mode != mode || ready.UID != 0 || ready.GID != 0 || !sourceCutObserverSHA(ready.SHA256) {
			return nil, nil, errors.New("one exact sealed file ready record required")
		}
		if err := sourcePhysicalBirthTriple(records, ready, "file"); err != nil {
			return nil, nil, err
		}
		body, fileInfo, err := readSourceRecoveryHeldPayload(filePath, ready)
		if err != nil || sourcePhysicalSHA(body) != ready.SHA256 || name == ProgramName && ready.SHA256 != p.canonical.ProgramSHA256 {
			return nil, nil, errors.Join(errors.New("actual original sealed file bytes/inode required"), err)
		}
		fileNative := fileInfo.Sys().(*syscall.Stat_t)
		observed = append(observed, sourcePhysicalSealedFile{Record: ready, Size: fileInfo.Size(), Changed: fileNative.Ctim, Modified: fileNative.Mtim})
	}
	after, statErr := f.Stat()
	named, namedErr := os.Lstat(path)
	if statErr != nil || namedErr != nil || !sameSourceFile(info, after) || !sameSourceFile(info, named) {
		return nil, nil, errors.Join(errors.New("original sealed directory changed during inventory"), statErr, namedErr)
	}
	if err := checkSourceInventory(path, names); err != nil {
		return nil, nil, err
	}
	raw, err = json.Marshal(observed)
	return raw, info, err
}

// Match one exact consecutive intent/born/ready triple against the actual
// native inode/access/hash. Sequence is derived from the original vector.
func sourcePhysicalBirthTriple(records []sourceRecord, expected sourceRecord, kind string) error {
	matches := 0
	for i, ready := range records {
		if ready.Path != expected.Path || ready.Phase != kind+"-ready" {
			continue
		}
		wanted := expected
		wanted.Sequence, wanted.Phase = i, kind+"-ready"
		if i < 2 || ready != wanted {
			return errors.New("original physical ready lineage differs")
		}
		wanted.Sequence, wanted.Phase = i-1, kind+"-born"
		if records[i-1] != wanted {
			return errors.New("original physical born lineage differs")
		}
		wanted.Sequence, wanted.Phase, wanted.Device, wanted.Inode = i-2, kind+"-intent", 0, 0
		if records[i-2] != wanted {
			return errors.New("original physical intent lineage differs")
		}
		matches++
	}
	if matches != 1 {
		return errors.New("one original physical birth triple required")
	}
	return nil
}
