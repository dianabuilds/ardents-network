//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Hash/native-image observations only. The actual account helper has already
// joined at both selected production call sites; this grants no account-birth
// provenance and makes no claim about interruption while useradd is writing.
type sourcePhysicalEarlyEntry struct {
	Path              string
	Present           bool
	Record            sourceRecord
	Size              int64
	Changed, Modified syscall.Timespec
}

func sourcePhysicalEarlyEntryFor(path string, file *sourceFile) sourcePhysicalEarlyEntry {
	entry := sourcePhysicalEarlyEntry{Path: path}
	if file == nil {
		return entry
	}
	native := file.info.Sys().(*syscall.Stat_t)
	entry.Present = true
	entry.Record = sourceRecord{Path: path, Device: uint64(native.Dev), Inode: native.Ino, SHA256: digest(file.raw),
		Mode: uint32(file.info.Mode().Perm()), UID: native.Uid, GID: native.Gid}
	entry.Size, entry.Changed, entry.Modified = file.info.Size(), native.Ctim, native.Mtim
	return entry
}

// accountRoot is fixed to /etc by the canonical observer. A native file-only
// fixture can supply its private stand directory; no profile can change it.
func sourcePhysicalEarlyAccountImage(records []sourceRecord, accountRoot string) (raw []byte, target os.FileInfo, returned error) {
	if len(records) != 24 && len(records) != 42 || records[0].Phase != sourceRecoverableInitialPhase ||
		records[22].Phase != "account-intent" || records[22].Path != "ardents-source" ||
		records[22].UID < 100 || records[22].UID > 999 || records[22].GID != records[22].UID {
		return nil, nil, errors.New("exact incomplete account or first-directory prefix required")
	}
	files := map[string]*sourceFile{}
	defer func() {
		for _, file := range files {
			if file != nil {
				returned = errors.Join(returned, file.file.Close())
				clear(file.raw)
			}
		}
	}()
	paths := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
	paths = append(paths, sourceAccountAuxiliaryPaths()...)
	var entries []sourcePhysicalEarlyEntry
	for _, path := range paths {
		file, err := readSourceAccountFile(filepath.Join(accountRoot, filepath.Base(path)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
		files[path] = file
		entries = append(entries, sourcePhysicalEarlyEntryFor(path, file))
	}
	for number, path := range paths[:4] {
		current, before := files[path], files[path+"-"]
		preimage := records[5+number]
		if current == nil || before == nil || preimage.Phase != "account-preimage" || preimage.Path != path ||
			digest(before.raw) != preimage.SHA256 || sourceAccountDelta(path, before.raw, current.raw, records[22].UID, records[22].GID) != nil {
			return nil, nil, errors.New("actual account delta must preserve each recorded original database")
		}
	}
	for _, entry := range entries[4:] {
		expected := entry.Path == "/etc/.pwd.lock" || strings.HasSuffix(entry.Path, "-")
		if entry.Present != expected || entry.Path == "/etc/.pwd.lock" && (entry.Size != 0 || entry.Record.Mode != 0600 || entry.Record.GID != 0) {
			return nil, nil, errors.New("actual fixed account auxiliary result differs")
		}
	}
	for _, record := range records {
		if record.Phase != "account-born" && record.Phase != "account-auxiliary-result" && record.Phase != "account-auxiliary-cleared" {
			continue
		}
		matched := false
		for _, entry := range entries {
			if entry.Path != record.Path {
				continue
			}
			matched = true
			if record.Phase == "account-auxiliary-cleared" {
				if entry.Present || record != (sourceRecord{Sequence: record.Sequence, Phase: record.Phase, Path: record.Path}) {
					return nil, nil, errors.New("recorded absent account auxiliary was adopted")
				}
			} else {
				expected := entry.Record
				expected.Sequence, expected.Phase = record.Sequence, record.Phase
				if !entry.Present || expected != record {
					return nil, nil, errors.New("actual account inode/bytes/access differ from recorded birth")
				}
			}
		}
		if !matched {
			return nil, nil, errors.New("foreign account result path")
		}
	}
	for _, file := range files {
		if file != nil {
			if err := checkSourceAccountFile(file); err != nil {
				return nil, nil, err
			}
		}
	}
	encoded, err := json.Marshal(entries)
	return encoded, files["/etc/group"].info, err
}

func sourcePhysicalEarlyDirectory(path string) (entry sourcePhysicalEarlyEntry, info os.FileInfo, returned error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
	if err != nil {
		return entry, nil, err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	info, err = file.Stat()
	if err != nil || !sourceAccess(info, 0700, 0, 0, true) {
		return entry, nil, errors.Join(errors.New("actual unrecorded root0700 directory birth required"), err)
	}
	if names, err := file.Readdirnames(1); len(names) != 0 || !errors.Is(err, io.EOF) {
		return entry, nil, errors.Join(errors.New("unrecorded original root must remain empty"), err)
	}
	named, err := os.Lstat(path)
	if err != nil || !sameSourceFile(info, named) {
		return entry, nil, errors.Join(errors.New("original directory inode/name changed"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	entry = sourcePhysicalEarlyEntry{Path: path, Present: true,
		Record: sourceRecord{Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0700},
		Size:   info.Size(), Changed: native.Ctim, Modified: native.Mtim}
	return entry, info, nil
}

func (p *sourcePhysicalCut) canonicalEarlyImage(records []sourceRecord, root string) ([]byte, os.FileInfo, error) {
	last := records[len(records)-1]
	if p.window == "account-before-complete-birth" {
		if len(records) != 24 || last.Phase != "account-born" || last.Path != "/etc/passwd" {
			return nil, nil, errors.New("exact first recorded account database birth required")
		}
		return sourcePhysicalEarlyAccountImage(records, "/etc")
	}
	if p.window != "directory-born-before-record" || len(records) != 42 || last.Phase != "directory-intent" || last.Path != root {
		return nil, nil, errors.New("exact first directory intent required")
	}
	account, _, err := sourcePhysicalEarlyAccountImage(records, "/etc")
	if err != nil {
		return nil, nil, err
	}
	entry, info, err := sourcePhysicalEarlyDirectory(root)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(struct {
		Account   json.RawMessage
		Directory sourcePhysicalEarlyEntry
	}{account, entry})
	return raw, info, err
}
