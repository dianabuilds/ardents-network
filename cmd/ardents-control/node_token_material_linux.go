package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

// prepareNodeTokenMaterial is the shared offline key-material ceremony. The
// role supplies its own argument grammar, plan schema and inventory schema;
// Admission still owns generation and storage of the separate immutable
// material root. No private material crosses this boundary, only Inventory.
//
// The fresh enclosing output prevents adoption of an existing or partial
// material root. Its original birth and native ancestry remain retained while
// Admission initializes and reopens its own separate immutable material root.
func prepareNodeTokenMaterial[P any, I any](ctx context.Context, args []string, observe func(string, *os.File), arguments func([]string) (P, issuance.Binding, string, error), newInventory func(issuance.Inventory) I) (public []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 {
		return nil, errSourceControlInput
	}
	p, binding, output, err := arguments(args)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var birth os.FileInfo
	retainBirth := func(phase string, file *os.File) {
		if phase == "directory-created" && birth == nil {
			birth, err = file.Stat()
		}
		if observe != nil {
			observe("plan-"+phase, file)
		}
	}
	if err := persistSourceArtifacts(ctx, output, []sourceArtifactFile{{"token-key-plan.json", append(raw, '\n')}}, ctx.Err, retainBirth); err != nil {
		return nil, err
	}
	if err != nil || birth == nil {
		return nil, errors.Join(errSourceControlInput, err)
	}
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if returned != nil {
			public = nil
		}
	}()
	directory, err := inputs.directory(output)
	if err != nil {
		return nil, err
	}
	if directory.info.Mode() != os.ModeDir|0700 || !os.SameFile(birth, directory.info) {
		return nil, errSourceControlInput
	}
	retainedPlan, err := inputs.read(filepath.Join(output, "token-key-plan.json"), 4096)
	if err != nil || string(retainedPlan) != string(append(raw, '\n')) {
		return nil, errors.Join(errSourceControlInput, err)
	}
	check := func() error { return errors.Join(inputs.checkProtected(), ctx.Err()) }
	if err := check(); err != nil {
		return nil, err
	}
	material := filepath.Join(output, "material")
	if err := issuance.Initialize(ctx, material, binding); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	store, err := issuance.Open(ctx, material, binding)
	if err != nil {
		return nil, err
	}
	// Physical Close joins before input descriptors and public acknowledgement.
	defer func() {
		returned = errors.Join(returned, store.Close())
		if observe != nil {
			observe("material-closed", directory.file)
		}
	}()
	if observe != nil {
		observe("material-open", directory.file)
	}
	if err := errors.Join(store.Check(), check()); err != nil {
		return nil, err
	}
	inventory, err := store.Inventory()
	if err != nil {
		return nil, err
	}
	public, err = json.Marshal(newInventory(inventory))
	if err != nil {
		return nil, err
	}
	public = append(public, '\n')
	retained := func() error { return errors.Join(store.Check(), check()) }
	if err := persistSourceArtifacts(ctx, filepath.Join(output, "public"), []sourceArtifactFile{{"token-inventory.json", public}}, retained, observe); err != nil {
		return nil, err
	}
	return public, retained()
}

// runNodeTokenMaterial renders one role's fixed offline material receipt. The
// role supplies its own operation prefix and failure classification; this
// grants no State, Permission or installed Issuer authority.
func runNodeTokenMaterial(role string, args []string, out io.Writer, fail func(error) error, prepare func(context.Context, []string, func(string, *os.File)) ([]byte, error)) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepare(ctx, args, nil)
	if err != nil {
		return fail(err)
	}
	sum := sha256.Sum256(public)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Issuer    bool   `json:"issuer_authority"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}{role + ".prepare-token-keys", hex.EncodeToString(sum[:]), false, false, false, false})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}
