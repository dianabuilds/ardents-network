//go:build linux

package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type startSocketRecord struct {
	Schema       string `json:"schema"`
	IntentDigest string `json:"intent_digest"`
	Device       uint64 `json:"device"`
	Inode        uint64 `json:"inode"`
	GID          uint32 `json:"gid"`
}

func recordStartSocket(root string, gid uint32) error {
	guard, err := readStartGuard(root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(filepath.Join(root, "start-completion.socket"))
	if err != nil {
		return err
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != 0 || identity.Gid != gid || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0660 {
		return errors.New("installation completion socket birth differs")
	}
	record := startSocketRecord{Schema: "ardents-endpoint-installation-start-socket-v1", IntentDigest: digestHex(guard), Device: uint64(identity.Dev), Inode: uint64(identity.Ino), GID: gid}
	body, err := canonicalJSON(record)
	if err != nil {
		return err
	}
	if err := writeExclusiveGenerationFile(filepath.Join(root, "start-socket.json"), body, 0600, 0); err != nil {
		return err
	}
	return syncDirectory(root)
}

func readStartSocketRecord(root string, gid uint32) (startSocketRecord, error) {
	path := filepath.Join(root, "start-socket.json")
	if err := requirePrivateJournalFile(path); err != nil {
		return startSocketRecord{}, err
	}
	body, err := readInstalledFile(path, 4096)
	if err != nil {
		return startSocketRecord{}, err
	}
	guard, err := readStartGuard(root)
	if err != nil {
		return startSocketRecord{}, err
	}
	var record startSocketRecord
	if err := decodeCanonical(body, 4096, &record); err != nil || record.Schema != "ardents-endpoint-installation-start-socket-v1" || record.IntentDigest != digestHex(guard) || record.Inode == 0 || record.GID != gid {
		return startSocketRecord{}, errors.New("installation completion socket record differs")
	}
	return record, nil
}

func validateStartSocket(root string, gid uint32) error {
	path := filepath.Join(root, "start-completion.socket")
	info, socketErr := os.Lstat(path)
	if socketErr != nil && !os.IsNotExist(socketErr) {
		return socketErr
	}
	_, recordErr := os.Lstat(filepath.Join(root, "start-socket.json"))
	if os.IsNotExist(recordErr) && os.IsNotExist(socketErr) {
		return nil
	}
	if recordErr != nil {
		return errors.Join(errors.New("installation completion socket has no owned birth record"), recordErr)
	}
	record, err := readStartSocketRecord(root, gid)
	if err != nil {
		return err
	}
	if socketErr == nil {
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || identity.Uid != 0 || identity.Gid != gid || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0660 || uint64(identity.Dev) != record.Device || uint64(identity.Ino) != record.Inode {
			return errors.New("installation completion socket recorded inode differs")
		}
	}
	return nil
}

func clearStartSocket(root string, gid uint32) error {
	if err := validateStartSocket(root, gid); err != nil {
		return err
	}
	path := filepath.Join(root, "start-completion.socket")
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := syncDirectory(root); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, "start-socket.json")); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(root, "start-socket.json")); err != nil {
		return err
	}
	return syncDirectory(root)
}
