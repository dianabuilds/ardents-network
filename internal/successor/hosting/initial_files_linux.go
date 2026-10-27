package hosting

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// FileBirths describes only original native files created by InitializeFiles.
// It grants no allowance, installation permission or live reservation.
type FileBirths struct{ originals map[string]os.FileInfo }

// Original returns the original observation for one fixed period file. The
// caller must match it to the actual descriptor before delivery or promotion.
func (births FileBirths) Original(name string) os.FileInfo { return births.originals[name] }

// InitializeFiles creates the period's files in a borrowed original empty
// directory. It neither creates nor replaces that directory and does not close
// the caller's handle. The installation caller must independently admit its
// known fresh birth, exclude concurrent users and durably prohibit another
// initialization outside the runtime-writable root before calling this method.
// A native directory handle is not that provenance or permission. Initialize
// remains the absent-path initializer; Open never initializes missing files.
// Any error after file creation is uncertain and leaves its original prefix.
func InitializeFiles(ctx context.Context, directory *os.Root, policy Policy) (FileBirths, error) {
	births := FileBirths{originals: map[string]os.FileInfo{}}
	if err := initializeFiles(ctx, directory, policy, births); err != nil {
		return FileBirths{}, err
	}
	return births, nil
}

func initializeFiles(ctx context.Context, directory *os.Root, policy Policy, births FileBirths) (returned error) {
	if ctx == nil || directory == nil {
		return ErrInvalid
	}
	if err := errors.Join(hostingPlatform(), policy.Validate(), ctx.Err()); err != nil {
		return err
	}
	reading, err := measureHosting(policy.Interfaces)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if now.Before(policy.Start) || !now.Before(policy.End) {
		return ErrInvalid
	}
	info, err := directory.Stat(".")
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return errors.Join(ErrInvalid, err)
	}
	probe, err := directory.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := probe.ReadDir(1)
	closeErr := probe.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if len(entries) != 0 {
		return errors.New("hosting initialization directory is not empty")
	}
	// From this point even a failed exclusive create retains uncertainty: a
	// competing writer or ambiguous syscall cannot be reclassified as fresh.
	defer func() {
		if returned != nil {
			returned = errors.Join(ErrUncertain, returned)
		}
	}()
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	pin := sha256.Sum256(policyBytes)
	if err := ctx.Err(); err != nil {
		return err
	}
	if births.originals["budget.pin"], err = createHostingOriginal(directory, "budget.pin", []byte(hex.EncodeToString(pin[:]))); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if births.originals["budget.lock"], err = createHostingOriginal(directory, "budget.lock", nil); err != nil {
		return err
	}
	state := hostingState{Schema: hostingSchema, Policy: policy, Used: policy.InitialUsedBytes, Observed: now, Reading: reading}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	envelope := struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}{body, hex.EncodeToString(sum[:])}
	raw, err := json.Marshal(envelope)
	if err != nil || len(raw) > 16<<10 {
		return errors.Join(ErrInvalid, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := createHostingFile(directory, "budget.pending", raw); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Exclusive initial creation cannot overwrite a file inserted after the
	// empty-directory observation. Ordinary later transactions retain Rename.
	if births.originals["budget.json"], err = createHostingOriginal(directory, "budget.json", raw); err != nil {
		return err
	}
	opened, err := directory.Open(".")
	if err != nil {
		return err
	}
	if err := errors.Join(opened.Sync(), opened.Close(), ctx.Err()); err != nil {
		return err
	}
	confirmed, err := readHostingFile(directory, "budget.json", 16<<10)
	if err != nil || !bytes.Equal(raw, confirmed) {
		return errors.Join(ErrUncertain, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := directory.Remove("budget.pending"); err != nil {
		return err
	}
	opened, err = directory.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(opened.Sync(), opened.Close(), ctx.Err())
}
