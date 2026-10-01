package resource

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const hostingSchema = "ardents-hosting-period-v1"

func openHostingRoot(path string) (*os.Root, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("hosting root is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0002 != 0 {
		return nil, errors.New("hosting root is unavailable")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.Join(errors.New("hosting root changed"), root.Close())
	}
	return root, nil
}

func initializeHosting(path string, policy HostingPolicy, reading hostingReading, now time.Time) error {
	if _, err := policy.limit(); err != nil {
		return err
	}
	if now.Before(policy.Start) || !now.Before(policy.End) || reading.Boot == "" || len(reading.Interfaces) != len(policy.Interfaces) {
		return errors.New("hosting initial observation is invalid")
	}
	for index, name := range policy.Interfaces {
		if reading.Interfaces[index].Name != name || reading.Interfaces[index].Index == 0 {
			return errors.New("hosting initial interface is invalid")
		}
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("hosting root is invalid")
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return errors.New("hosting period already exists or cannot be initialized")
	}
	root, err := openHostingRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(policyBytes)
	if err := createHostingFile(root, "period.pin", []byte(hex.EncodeToString(digest[:]))); err != nil {
		return err
	}
	if err := createHostingFile(root, "period.lock", nil); err != nil {
		return err
	}
	state := hostingState{Schema: hostingSchema, Policy: policy, Used: policy.InitialUsedBytes, Observed: now, Reading: reading}
	if err := writeHostingState(root, state); err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(parent.Sync(), parent.Close())
}

func createHostingFile(root *os.Root, name string, body []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	count, writeErr := file.Write(body)
	if writeErr == nil && count != len(body) {
		writeErr = io.ErrShortWrite
	}
	syncErr := file.Sync()
	return errors.Join(writeErr, syncErr, file.Close())
}

func readHostingFile(root *os.Root, name string, maximum int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maximum {
		return nil, errors.New("hosting state file is unavailable")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(errors.New("hosting state file changed"), file.Close())
	}
	body, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(body)) != before.Size() {
		return nil, errors.New("hosting state read is ambiguous")
	}
	return body, nil
}

func readHostingState(root *os.Root) (hostingState, error) {
	if _, err := root.Lstat("period.pending"); !errors.Is(err, os.ErrNotExist) {
		return hostingState{}, errors.New("hosting state has an unfinished write")
	}
	return readCommittedHostingState(root)
}

func readCommittedHostingState(root *os.Root) (hostingState, error) {
	var state hostingState
	raw, err := readHostingFile(root, "period.json", 16<<10)
	if err != nil {
		return state, err
	}
	var envelope struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}
	if err := decodeHostingCanonical(raw, &envelope); err != nil {
		return state, err
	}
	digest := sha256.Sum256(envelope.State)
	if envelope.SHA256 != hex.EncodeToString(digest[:]) {
		return state, errors.New("hosting state checksum differs")
	}
	if err := decodeHostingCanonical(envelope.State, &state); err != nil {
		return state, err
	}
	limit, err := state.Policy.limit()
	if err != nil || state.Schema != hostingSchema || state.Used < state.Policy.InitialUsedBytes || state.Reserved > limit ||
		state.Observed.Before(state.Policy.Start) || state.Reading.Boot == "" || len(state.Reading.Interfaces) != len(state.Policy.Interfaces) {
		return state, errors.New("hosting state is invalid")
	}
	for index, name := range state.Policy.Interfaces {
		if state.Reading.Interfaces[index].Name != name || state.Reading.Interfaces[index].Index == 0 {
			return state, errors.New("hosting state interface differs")
		}
	}
	pin, err := readHostingFile(root, "period.pin", 64)
	if err != nil {
		return state, err
	}
	policyBytes, err := json.Marshal(state.Policy)
	if err != nil {
		return state, err
	}
	policyDigest := sha256.Sum256(policyBytes)
	if string(pin) != hex.EncodeToString(policyDigest[:]) {
		return state, errors.New("hosting policy binding differs")
	}
	return state, nil
}

func decodeHostingCanonical(raw []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return errors.New("hosting state encoding is invalid")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return errors.New("hosting state has trailing data")
	}
	canonical, err := json.Marshal(output)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("hosting state is not canonical")
	}
	return nil
}

func writeHostingState(root *os.Root, state hostingState) error {
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	envelope := struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}{State: body, SHA256: hex.EncodeToString(digest[:])}
	raw, err := json.Marshal(envelope)
	if err != nil || len(raw) > 16<<10 {
		return errors.New("hosting state exceeds its bound")
	}
	if err := createHostingFile(root, "period.pending", raw); err != nil {
		return err
	}
	if err := root.Rename("period.pending", "period.json"); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		return err
	}
	confirmed, err := readHostingFile(root, "period.json", 16<<10)
	if err != nil || !bytes.Equal(raw, confirmed) {
		return errors.New("hosting commit could not be confirmed")
	}
	return nil
}
