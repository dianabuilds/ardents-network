//go:build ignore

// Command sign-qualification-network signs one bounded initial Node Record or
// Epoch under ADR-0120. It never admits State or installs a process.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"golang.org/x/sys/unix"
)

func main() {
	if err := signNetwork(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func signNetwork() error {
	if os.Geteuid() != 0 || len(os.Args) != 4 {
		return errors.New("requires Linux root: public-plan private-key new-output-directory")
	}
	output := os.Args[3]
	if !filepath.IsAbs(output) || filepath.Clean(output) != output || output == "/" {
		return errors.New("qualification Network output path is invalid")
	}
	if err := secureNetworkAncestors(filepath.Dir(output)); err != nil {
		return err
	}
	raw, err := readNetworkInput(os.Args[1], 8<<20, false)
	if err != nil {
		return err
	}
	var plan struct {
		Schema    string                     `json:"schema"`
		Reference time.Time                  `json:"reference_time"`
		Record    *epoch.InitialClosedRecord `json:"record"`
		Epoch     *epoch.InitialClosedEpoch  `json:"epoch"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Schema != "ardents-qualification-network-signing-plan-v1" || (plan.Record == nil) == (plan.Epoch == nil) {
		return errors.New("initial Network signing plan is invalid")
	}
	now := time.Now().UTC()
	if plan.Reference.IsZero() || plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) {
		return errors.New("qualification Network signing reference time is invalid")
	}
	keyBytes, err := readNetworkInput(os.Args[2], 16<<10, true)
	if err != nil {
		return err
	}
	defer clear(keyBytes)
	block, rest := pem.Decode(keyBytes)
	if block == nil || len(rest) != 0 || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 {
		return errors.New("qualification Network signing key encoding is invalid")
	}
	defer clear(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || len(key) != ed25519.PrivateKeySize {
		return errors.New("qualification Network signing key is not Ed25519")
	}
	defer clear(key)
	public := key.Public().(ed25519.PublicKey)
	outputs := map[string][]byte{}
	operation := "node-record"
	var signed []byte
	if plan.Record != nil {
		input := *plan.Record
		if !bytes.Equal(input.PublicKey, public) || !validSigningInterval(input.ValidFrom, input.ValidUntil, plan.Reference, now) {
			return errors.New("initial Node Record signer or validity mismatch")
		}
		message, err := epoch.PrepareInitialClosedRecord(input)
		if err != nil {
			return err
		}
		signed = append(message, ed25519.Sign(key, message)...)
		if !ed25519.Verify(public, message, signed[len(message):]) {
			return errors.New("initial Node Record signature verification failed")
		}
		outputs["node-record.bin"] = signed
	} else {
		operation = "epoch"
		input := *plan.Epoch
		if len(input.AuthorityKeys) != 1 || !bytes.Equal(input.AuthorityKeys[0], public) || !validSigningInterval(input.ValidFrom, input.ValidUntil, plan.Reference, now) {
			return errors.New("initial Epoch signer or validity mismatch")
		}
		message, err := epoch.PrepareInitialClosedEpoch(input)
		if err != nil {
			return err
		}
		digest, id := sha256.Sum256(message), sha256.Sum256(public)
		signed = append(message, 1)
		signed = append(signed, id[:]...)
		signed = append(signed, ed25519.Sign(key, digest[:])...)
		policy := epoch.Policy{NetworkID: input.NetworkID, Authorities: map[[32]byte]ed25519.PublicKey{id: public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: now}
		decision, err := epoch.Verify(policy, signed, input.Records, nil, false)
		if err != nil || len(decision.Candidates) != len(input.Records) {
			return errors.New("initial Epoch ordinary verification failed")
		}
		materials := make([][]byte, len(decision.Candidates))
		for index := range materials {
			materials[index], err = decision.Materialization(uint32(index))
			if err != nil {
				return err
			}
			outputs[fmt.Sprintf("material-%02d.bin", index)] = materials[index]
		}
		if _, err := epoch.Verify(policy, signed, input.Records, materials, true); err != nil {
			return errors.New("initial Epoch material verification failed")
		}
		for index, record := range input.Records {
			outputs[fmt.Sprintf("input-%02d.bin", index)] = record
		}
		outputs["epoch.bin"] = signed
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return errors.New("qualification Network output creation refused; existing output is never replaced")
	}
	for name, value := range outputs {
		if err := writeNetworkOutput(filepath.Join(output, name), value); err != nil {
			return err
		}
	}
	for _, path := range []string{output, filepath.Dir(output)} {
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := errors.Join(dir.Sync(), dir.Close()); err != nil {
			return err
		}
	}
	digest := sha256.Sum256(signed)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "ardents-qualification-network-signing-receipt-v1", "operation": operation, "signed_sha256": hex.EncodeToString(digest[:]), "signatures_verified": true, "state_accepted": false, "installed_qualification": false})
}

func validSigningInterval(from, until, reference, now time.Time) bool {
	return !reference.Before(from) && reference.Before(until) && !now.Before(from) && now.Before(until) && until.Sub(from) <= 7*24*time.Hour
}
func secureNetworkAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("qualification Network private path is invalid")
	}
	for {
		var stat unix.Stat_t
		if unix.Lstat(path, &stat) != nil || stat.Uid != 0 || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0022 != 0 {
			return errors.New("qualification Network ancestor must be root-owned non-writable direct directory")
		}
		if path == "/" {
			return nil
		}
		path = filepath.Dir(path)
	}
}

func readNetworkInput(path string, limit int64, private bool) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || limit < 1 {
		return nil, errors.New("qualification Network input path or limit is invalid")
	}
	if err := secureNetworkAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("qualification Network input open refused")
	}
	file := os.NewFile(uintptr(fd), path)
	info, statErr := file.Stat()
	var stat unix.Stat_t
	ownershipErr := unix.Fstat(fd, &stat)
	if statErr != nil || ownershipErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit || (private && (stat.Uid != 0 || info.Mode().Perm() != 0600)) {
		return nil, errors.Join(errors.New("qualification Network input is not a bounded direct file with required ownership"), statErr, ownershipErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if int64(len(raw)) > limit {
		clear(raw)
		return nil, errors.Join(errors.New("qualification Network input exceeds opened-file bound"), readErr, closeErr)
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		clear(raw)
		return nil, err
	}
	return raw, nil
}

func writeNetworkOutput(path string, value []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(value)
	if writeErr == nil && n != len(value) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Sync(), file.Close())
}
