//go:build ignore

// Command sign-qualification-alpha signs the fixed initial disclosure set
// under four separate keys. It never asserts Release or State acceptance.
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

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"golang.org/x/sys/unix"
)

func main() {
	if err := signAlpha(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func signAlpha() error {
	if os.Geteuid() != 0 || len(os.Args) != 4 {
		return errors.New("requires Linux root: public-plan private-key-directory new-output-directory")
	}
	keyRoot, output := os.Args[2], os.Args[3]
	if err := secureAlphaAncestors(keyRoot); err != nil {
		return err
	}
	if !filepath.IsAbs(output) || filepath.Clean(output) != output || output == "/" {
		return errors.New("alpha output path is invalid")
	}
	if err := secureAlphaAncestors(filepath.Dir(output)); err != nil {
		return err
	}
	raw, err := readAlphaInput(os.Args[1], 32<<20, false)
	if err != nil {
		return err
	}
	var plan struct {
		Schema        string                           `json:"schema"`
		Cohort        string                           `json:"cohort"`
		Reference     time.Time                        `json:"reference_time"`
		NotBefore     time.Time                        `json:"not_before"`
		NotAfter      time.Time                        `json:"not_after"`
		Release       inspection.ReleaseEvidence       `json:"release"`
		Network       inspection.NetworkEvidence       `json:"network"`
		Compatibility inspection.CompatibilityEvidence `json:"compatibility"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Schema != "ardents-qualification-alpha-signing-plan-v1" {
		return errors.New("alpha signing plan is invalid")
	}
	header, inspectErr := epoch.Inspect(plan.Network.Epoch)
	if inspectErr != nil || header.Version != 3 || header.Number != 1 || header.Previous != [32]byte{} || plan.Network.Profile != epoch.ProfileClosedRoute || plan.Release.TargetPath != "ardents/linux-amd64/endpoint" || plan.Release.ProtocolPhase != "announced" || plan.Release.BuildState != "current" {
		return errors.New("alpha signing requires the initial current closed Route disclosure")
	}
	now := time.Now().UTC()
	if plan.Reference.IsZero() || plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) || plan.Reference.Before(plan.NotBefore) || !plan.Reference.Before(plan.NotAfter) || !plan.NotAfter.After(now) || plan.NotAfter.Sub(plan.NotBefore) > 7*24*time.Hour {
		return errors.New("alpha signing validity is invalid")
	}
	bodies, err := inspection.PrepareInitialEvidence(plan.Release, plan.Network, plan.Compatibility)
	if err != nil {
		return err
	}
	roles := []string{"catalog", "release", "network", "compatibility"}
	var keys [4]ed25519.PrivateKey
	defer func() {
		for _, key := range keys {
			clear(key)
		}
	}()
	var public [4]ed25519.PublicKey
	for index, role := range roles {
		encoded, err := readAlphaInput(filepath.Join(keyRoot, role+"-key.pem"), 16<<10, true)
		if err != nil {
			return err
		}
		block, rest := pem.Decode(encoded)
		if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
			clear(encoded)
			return errors.New("alpha key encoding is invalid")
		}
		value, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		clear(block.Bytes)
		clear(encoded)
		if parseErr != nil {
			return errors.New("alpha private key parsing failed")
		}
		key, ok := value.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return errors.New("alpha private key type is invalid")
		}
		keys[index] = key
		public[index] = key.Public().(ed25519.PublicKey)
		for previous := 0; previous < index; previous++ {
			if bytes.Equal(public[index], public[previous]) {
				return errors.New("alpha role keys are duplicated")
			}
		}
	}
	var components [3][]byte
	catalog := alphacontrol.Catalog{Cohort: plan.Cohort, Generation: 1, NotBefore: plan.NotBefore, NotAfter: plan.NotAfter}
	for index, body := range bodies {
		input := alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1, NotBefore: plan.NotBefore, NotAfter: plan.NotAfter, Body: body}
		message, err := alphacontrol.PrepareInitialComponentSigningInput(input)
		if err != nil {
			return err
		}
		components[index], err = signAlphaMessage(message, "ardents-alpha-control-component-v1\x00", keys[index+1])
		if err != nil {
			return err
		}
		catalog.Components[index] = alphacontrol.Component{Class: input.Class, RootID: sha256.Sum256(public[index+1]), Generation: 1, NotAfter: plan.NotAfter, Size: uint32(len(components[index])), Digest: sha256.Sum256(components[index])}
	}
	message, err := alphacontrol.PrepareInitialCatalogSigningInput(catalog)
	if err != nil {
		return err
	}
	catalogBytes, err := signAlphaMessage(message, "ardents-alpha-control-catalog-v1\x00", keys[0])
	if err != nil {
		return err
	}
	var callbacks [3]bool
	result, _, err := alphacontrol.Inspect(catalogBytes, public[0], [3]ed25519.PublicKey{public[1], public[2], public[3]}, components, alphacontrol.Floor{}, plan.Reference, func(component alphacontrol.Component, statement alphacontrol.ComponentStatement, _ time.Time) alphacontrol.Outcome {
		index := int(component.Class) - 1
		if index >= 0 && index < 3 && bytes.Equal(statement.Body, bodies[index]) {
			callbacks[index] = true
		}
		return alphacontrol.OutcomeUnavailable
	})
	if err != nil || result.Catalog != alphacontrol.OutcomeAccepted || callbacks != [3]bool{true, true, true} {
		return errors.Join(errors.New("signed alpha envelope verification failed"), err)
	}
	retired, err := readAlphaInput(filepath.Join(keyRoot, "corpus.pub"), 32, false)
	if err != nil || len(retired) != 32 {
		return errors.Join(errors.New("retired public companion is invalid"), err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		return errors.New("alpha output exists or cannot be created")
	}
	files := map[string][]byte{"catalog.ac1": catalogBytes, "catalog.pub": public[0], "corpus.pub": retired}
	for index, name := range []string{"release", "network", "compatibility"} {
		files[name+".ac1"] = components[index]
		files[name+".pub"] = public[index+1]
	}
	for name, value := range files {
		if err := writeAlphaOutput(filepath.Join(output, name), value); err != nil {
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
	digest := sha256.Sum256(catalogBytes)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": "ardents-qualification-alpha-signing-receipt-v1", "catalog_sha256": hex.EncodeToString(digest[:]), "envelope_signatures_verified": true, "release_state_acceptance": false, "installed_qualification": false})
}

func signAlphaMessage(message []byte, domain string, key ed25519.PrivateKey) ([]byte, error) {
	if !bytes.HasPrefix(message, []byte(domain)) || len(message) <= len(domain) {
		return nil, errors.New("alpha signing domain is invalid")
	}
	return append(append([]byte(nil), message[len(domain):]...), ed25519.Sign(key, message)...), nil
}

func secureAlphaAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("alpha private path is invalid")
	}
	for {
		var stat unix.Stat_t
		if unix.Lstat(path, &stat) != nil || stat.Uid != 0 || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0022 != 0 {
			return errors.New("alpha ancestor must be root-owned non-writable direct directory")
		}
		if path == "/" {
			return nil
		}
		path = filepath.Dir(path)
	}
}

func readAlphaInput(path string, limit int64, private bool) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || limit < 1 {
		return nil, errors.New("alpha input path or limit is invalid")
	}
	if err := secureAlphaAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("alpha input open refused")
	}
	file := os.NewFile(uintptr(fd), path)
	info, statErr := file.Stat()
	var stat unix.Stat_t
	ownershipErr := unix.Fstat(fd, &stat)
	if statErr != nil || ownershipErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit || (private && (stat.Uid != 0 || info.Mode().Perm() != 0600)) {
		return nil, errors.Join(errors.New("alpha input is not a bounded direct file with required ownership"), statErr, ownershipErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if int64(len(raw)) > limit {
		clear(raw)
		return nil, errors.Join(errors.New("alpha input exceeds opened-file bound"), readErr, closeErr)
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		clear(raw)
		return nil, err
	}
	return raw, nil
}

func writeAlphaOutput(path string, value []byte) error {
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
