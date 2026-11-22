package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// A finite operator plan rejects implicit last-wins option selection. Only
// the exact options named by its caller may be repeated. This is the single
// public-option grammar for every Forwarding/JOIN/Descriptor record, profile,
// token-material, release-plan and Epoch argument parser; the Epoch producers
// pass domain/record-plan/record (#718, audit E1-3). The option predicate
// matches Go flag's own parseOne: a bare "-" is not an option.
// Public declarations and existing artifact bytes only. This operation opens
// no key directory and creates only the selected role's public plan.
func nodeReleasePlanArguments(role nodeReleaseRole, args []string, inputs *artifactio.Inputs, purpose nodeReleasePurpose) ([]byte, string, error) {
	schema := role.schema(purpose)
	if inputs == nil || schema == "" {
		return nil, "", artifactio.ErrInput
	}
	set := flag.NewFlagSet(string(role)+" prepare-release-plan", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	p := fixedNodeReleasePlan{Schema: schema}
	var reference, expires, programCustom, generationCustom, destination string
	set.StringVar(&p.Network, "network", "", "approved software network declaration")
	set.StringVar(&reference, "reference", "", "current UTC RFC3339 time")
	set.StringVar(&expires, "expires", "", "UTC RFC3339 expiry")
	set.StringVar(&p.Program, "program", "", "exact canonical Node program")
	set.StringVar(&p.Descriptor, "descriptor", "", "exact role descriptor")
	set.StringVar(&p.Template, "template", "", "fixed role template")
	set.StringVar(&programCustom, "program-custom", "", "reviewed public program claims")
	set.StringVar(&generationCustom, "generation-custom", "", "reviewed public generation claims")
	set.StringVar(&destination, "output", "", "fresh output directory")
	if set.Parse(args) != nil || set.NArg() != 0 || set.NFlag() != 9 || !artifactio.UniqueOptions(args, "") || !artifactio.AbsolutePath(destination) {
		return nil, "", artifactio.ErrInput
	}
	var err error
	p.Reference, err = time.Parse(time.RFC3339, reference)
	if err != nil {
		return nil, "", err
	}
	p.Expires, err = time.Parse(time.RFC3339, expires)
	if err != nil {
		return nil, "", err
	}
	p.ProgramCustom, err = inputs.Read(programCustom, 1<<20)
	if err != nil {
		return nil, "", err
	}
	p.GenerationCustom, err = inputs.Read(generationCustom, 1<<20)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	if _, err = decodeNodeReleasePlan(role, raw, time.Now().UTC(), purpose); err != nil {
		return nil, "", err
	}
	var files [3][]byte
	for i, path := range []string{p.Program, p.Descriptor, p.Template} {
		maximum := int64(64 << 20)
		if i == 1 {
			maximum = 16 << 10
		}
		files[i], err = inputs.Read(path, maximum)
		if err != nil {
			return nil, "", err
		}
	}
	if _, err = validateNodeReleaseResources(role, p, files[0], files[1], files[2]); err != nil {
		return nil, "", err
	}
	return append(raw, '\n'), destination, nil
}

func runNodeReleasePlan(role nodeReleaseRole, args []string, out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, err := writeNodeReleasePlan(role, ctx, args)
	if err != nil {
		return nodeReleaseFailure(role, err)
	}
	sum := sha256.Sum256(raw)
	receipt, err := json.Marshal(struct {
		Operation string `json:"operation"`
		SHA256    string `json:"sha256"`
	}{string(role) + "." + args[0], hex.EncodeToString(sum[:])})
	if err != nil {
		return err
	}
	return artifactio.WriteReceipt(ctx, out, append(receipt, '\n'))
}

func writeNodeReleasePlan(role nodeReleaseRole, ctx context.Context, args []string) (result []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) == 0 {
		return nil, artifactio.ErrInput
	}
	var purpose nodeReleasePurpose
	switch args[0] {
	case "prepare-release-plan":
		purpose = nodeReleaseInitial
	case "prepare-forward-release-plan":
		purpose = nodeReleaseForward
	default:
		return nil, artifactio.ErrInput
	}
	inputs := artifactio.NewInputs(ctx)
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if returned != nil {
			result = nil
		}
	}()
	result, destination, err := nodeReleasePlanArguments(role, args[1:], inputs, purpose)
	if err != nil {
		return nil, err
	}
	files := []artifactio.Content{{Name: string(role) + "-release-plan.json", Data: result}}
	if err := artifactio.Publish(ctx, destination, files, inputs.CheckProtected, nil); err != nil {
		return nil, err
	}
	return bytes.Clone(result), inputs.CheckProtected()
}
