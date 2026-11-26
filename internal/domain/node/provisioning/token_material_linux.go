package provisioning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
)

// This is an offline key-material request, not an issuer appointment, signed
// profile, Permission or runtime plan. Admission owns generation and storage.
type nodeTokenMaterialPlan struct {
	Schema  string    `json:"schema"`
	Network string    `json:"network_id"`
	Issuer  string    `json:"issuer_node_id"`
	Signer  string    `json:"issuer_node_key"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

type nodeTokenInventory struct {
	Schema    string             `json:"schema"`
	Inventory issuance.Inventory `json:"inventory"`
}

func nodeTokenMaterialArguments(role nodeProvisioningRole, args []string) (p nodeTokenMaterialPlan, binding issuance.Binding, output string, returned error) {
	if !role.valid() || len(args) > 16 {
		return p, binding, output, artifacts.ErrInput
	}
	p.Schema = role.schema("token-key-plan")
	flags := flag.NewFlagSet(string(role)+" prepare-token-keys", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var start, end string
	flags.StringVar(&p.Network, "network-id", "", "explicit Network ID")
	flags.StringVar(&p.Issuer, "issuer-node-id", "", "explicit issuer Node ID")
	flags.StringVar(&p.Signer, "issuer-node-key", "", "explicit public issuer Node key")
	flags.StringVar(&start, "start", "", "first aligned UTC hour")
	flags.StringVar(&end, "end", "", "exclusive aligned UTC hour")
	flags.StringVar(&output, "output", "", "fresh protected offline material output")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 6 || !artifacts.UniqueOptions(args, "") || !artifacts.AbsolutePath(output) {
		return p, binding, output, artifacts.ErrInput
	}
	var err error
	for _, pair := range []struct {
		text   string
		target *[32]byte
	}{{p.Network, &binding.Network}, {p.Issuer, &binding.Issuer}, {p.Signer, &binding.Signer}} {
		*pair.target, err = controlPublic32(pair.text)
		if err != nil {
			return p, binding, output, err
		}
	}
	p.Start, err = time.Parse(time.RFC3339, start)
	if err != nil || p.Start.Location() != time.UTC || p.Start.Format(time.RFC3339) != start || p.Start.Unix() < 0 || p.Start.Unix()%3600 != 0 || p.Start.Nanosecond() != 0 {
		return p, binding, output, artifacts.ErrInput
	}
	p.End, err = time.Parse(time.RFC3339, end)
	if err != nil || p.End.Location() != time.UTC || p.End.Format(time.RFC3339) != end || p.End.Unix()%3600 != 0 || p.End.Nanosecond() != 0 || !p.End.After(p.Start) || p.End.Sub(p.Start) > 6*time.Hour {
		return p, binding, output, artifacts.ErrInput
	}
	binding.Start, binding.End = p.Start, p.End
	return p, binding, output, nil
}

// The fresh enclosing output prevents adoption of an existing or partial
// material root. Its original birth and native ancestry remain retained while
// Admission initializes and reopens its own separate immutable material root.
// No private material leaves that owner; only Inventory crosses this boundary.
func prepareNodeTokenMaterial(ctx context.Context, role nodeProvisioningRole, args []string, observe func(string, *os.File)) (public []byte, returned error) {
	if !role.valid() || ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 {
		return nil, artifacts.ErrInput
	}
	p, binding, output, err := nodeTokenMaterialArguments(role, args)
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
	if err := artifacts.Publish(ctx, output, []artifacts.Content{{Name: "token-key-plan.json", Data: append(raw, '\n')}}, ctx.Err, retainBirth); err != nil {
		return nil, err
	}
	if err != nil || birth == nil {
		return nil, errors.Join(artifacts.ErrInput, err)
	}
	inputs := artifacts.NewInputs(ctx)
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if returned != nil {
			public = nil
		}
	}()
	directory, err := inputs.Directory(output)
	if err != nil {
		return nil, err
	}
	if directory.Info().Mode() != os.ModeDir|0700 || !os.SameFile(birth, directory.Info()) {
		return nil, artifacts.ErrInput
	}
	retainedPlan, err := inputs.Read(filepath.Join(output, "token-key-plan.json"), 4096)
	if err != nil || string(retainedPlan) != string(append(raw, '\n')) {
		return nil, errors.Join(artifacts.ErrInput, err)
	}
	check := func() error { return errors.Join(inputs.CheckProtected(), ctx.Err()) }
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
	// Physical Close finishes before input descriptors and public acknowledgement.
	defer func() {
		returned = errors.Join(returned, store.Close())
		if observe != nil {
			observe("material-closed", directory.Descriptor())
		}
	}()
	if observe != nil {
		observe("material-open", directory.Descriptor())
	}
	if err := errors.Join(store.Check(), check()); err != nil {
		return nil, err
	}
	inventory, err := store.Inventory()
	if err != nil {
		return nil, err
	}
	public, err = json.Marshal(nodeTokenInventory{role.schema("token-inventory"), inventory})
	if err != nil {
		return nil, err
	}
	public = append(public, '\n')
	retained := func() error { return errors.Join(store.Check(), check()) }
	if err := artifacts.Publish(ctx, filepath.Join(output, "public"), []artifacts.Content{{Name: "token-inventory.json", Data: public}}, retained, observe); err != nil {
		return nil, err
	}
	return public, retained()
}

func runNodeTokenMaterial(role nodeProvisioningRole, args []string, out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareNodeTokenMaterial(ctx, role, args, nil)
	if err != nil {
		return role.failure(err)
	}
	sum := sha256.Sum256(public)
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA       string `json:"sha256"`
		State     bool   `json:"state_accepted"`
		Issuer    bool   `json:"issuer_authority"`
		Runtime   bool   `json:"runtime_authority"`
		Installed bool   `json:"installed_qualification"`
	}{role.operation("prepare-token-keys"), hex.EncodeToString(sum[:]), false, false, false, false})
	if err != nil {
		return err
	}
	return artifacts.WriteReceipt(ctx, out, append(raw, '\n'))
}
