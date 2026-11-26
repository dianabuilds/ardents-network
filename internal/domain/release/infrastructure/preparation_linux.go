//go:build linux

package infrastructure

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// PrepareRuntimeDescriptorFile reads and retains the original program and
// template until their public descriptor is written, reobserved and closed.
func PrepareRuntimeDescriptorFile(ctx context.Context, role, identity string, version int64, program, template, destination string, observe func(string, *os.File)) (result []byte, err error) {
	name, _, _, ok := runtimeNames(role)
	if ctx == nil || os.Geteuid() != 0 || !ok || !preparationPath(destination) {
		return nil, ErrInput
	}
	inputs := runtimePreparation{ctx: ctx}
	defer func() {
		result, err = inputs.finish(result, err)
	}()
	programBytes, err := inputs.read(program, 64<<20)
	if err != nil {
		return nil, err
	}
	templateBytes, err := inputs.read(template, 64<<20)
	if err != nil {
		return nil, err
	}
	raw, err := PrepareRuntimeDescriptor(role, identity, version, programBytes, templateBytes)
	if err != nil {
		return nil, err
	}
	return inputs.publish(destination, name, raw, observe)
}

// PrepareRuntimeInventoryFile checks the exact public distribution and retains
// its originals through publication of the new inventory. Trust and signing
// remain separate operations.
func PrepareRuntimeInventoryFile(ctx context.Context, role, distribution, destination string, observe func(string, *os.File)) (result []byte, err error) {
	descriptor, unit, _, ok := runtimeNames(role)
	if ctx == nil || os.Geteuid() != 0 || !ok || !preparationPath(distribution) || !preparationPath(destination) ||
		distribution == destination || strings.HasPrefix(destination, distribution+"/") {
		return nil, ErrInput
	}
	directory, err := artifacts.OpenDirectoryChain(ctx, distribution, nil)
	if err != nil {
		return nil, err
	}
	inputs := runtimePreparation{
		ctx:          ctx,
		distribution: directory,
		expected: map[string]int64{
			ProgramName:      64 << 20,
			unit:             64 << 20,
			descriptor:       16 << 10,
			"root.json":      1 << 20,
			"timestamp.json": 1 << 20,
			"snapshot.json":  1 << 20,
			"targets.json":   1 << 20,
		},
	}
	defer func() {
		result, err = inputs.finish(result, err)
	}()
	if err := inputs.checkDistribution(); err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(inputs.expected))
	for name, limit := range inputs.expected {
		files[name], err = inputs.read(filepath.Join(distribution, name), limit)
		if err != nil {
			return nil, err
		}
	}
	facts, err := ValidateRuntimeResources(role, files[descriptor], files[ProgramName], files[unit])
	if err != nil {
		return nil, err
	}
	raw, err := enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{
		Platform: facts.Platform, ReleaseIdentity: facts.ReleaseIdentity,
		ReleaseVersion: facts.ReleaseVersion, Invocation: facts.Invocation,
	}, files)
	if err != nil {
		return nil, err
	}
	return inputs.publish(destination, enrollment.InfrastructureInventoryName, raw, observe)
}

type runtimePreparation struct {
	ctx          context.Context
	files        []*artifacts.ProtectedInput
	distribution *artifacts.DirectoryChain
	expected     map[string]int64
}

func (p *runtimePreparation) read(path string, maximum int64) ([]byte, error) {
	if !preparationPath(path) {
		return nil, ErrInput
	}
	input, err := artifacts.OpenProtectedInput(p.ctx, path, maximum, false)
	if err != nil {
		return nil, err
	}
	p.files = append(p.files, input)
	if err := artifacts.RequireExt4(input.Descriptor()); err != nil {
		return nil, err
	}
	return input.Bytes(), nil
}

func (p *runtimePreparation) checkDistribution() error {
	if p.distribution == nil {
		return nil
	}
	if err := p.distribution.CheckRootAccess(); err != nil {
		return err
	}
	directory := p.distribution.Descriptor()
	if _, err := directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	names, err := directory.Readdirnames(len(p.expected) + 1)
	if err != nil && err != io.EOF || len(names) != len(p.expected) {
		return errors.Join(ErrBinding, err)
	}
	for _, name := range names {
		if _, ok := p.expected[name]; !ok {
			return ErrBinding
		}
	}
	return nil
}

func (p *runtimePreparation) checkIdentities() error {
	result := p.checkDistribution()
	// Later reads cannot vouch for an earlier input's identity through handoff.
	for _, file := range p.files {
		result = errors.Join(result, file.CheckIdentity(p.ctx))
	}
	return errors.Join(result, p.ctx.Err())
}

func (p *runtimePreparation) check() error {
	var result error
	for _, file := range p.files {
		result = errors.Join(result, file.Check(p.ctx))
	}
	return errors.Join(result, p.checkIdentities())
}

func (p *runtimePreparation) publish(destination, name string, raw []byte, observe func(string, *os.File)) ([]byte, error) {
	if err := p.check(); err != nil {
		return nil, err
	}
	if err := artifacts.Publish(p.ctx, destination, []artifacts.Content{{Name: name, Data: raw}}, p.check, observe); err != nil {
		return nil, err
	}
	return raw, p.checkIdentities()
}

func (p *runtimePreparation) finish(result []byte, err error) ([]byte, error) {
	if p.distribution != nil {
		err = errors.Join(err, p.distribution.Close())
	}
	for i := len(p.files) - 1; i >= 0; i-- {
		err = errors.Join(err, p.files[i].Close())
	}
	err = errors.Join(err, p.ctx.Err())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func preparationPath(path string) bool {
	return artifacts.AbsolutePath(path) && strings.Count(path, "/") < 128
}
