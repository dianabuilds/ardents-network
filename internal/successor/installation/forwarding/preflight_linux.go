package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// InspectInitialInputs is an independent root-only, read-only native preflight.
// It consumes no Release right, creates no resource and grants no later effect
// or startup admission. The actual creator must independently repeat its checks.
// Original inputs remain owned by the caller through this joined observation.
func InspectInitialInputs(input Inputs) (returned error) {
	p := input.original
	if p == nil {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	defer func() { p.record(returned) }()
	manager, err := systemd.OpenForwardingStopped(p.ctx)
	if manager != nil {
		defer func() { returned = errors.Join(returned, manager.Close()) }()
	}
	if err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		paths, err := manager.CheckAbsent()
		if err != nil {
			return err
		}
		if err := input.observeFresh(paths); err != nil {
			return err
		}
		if err := input.Check(); err != nil {
			return err
		}
	}
	return manager.CheckOriginal()
}

// observeFresh retains actual existing ancestors before testing absent leaves.
// It is serialized with original input reads; no manager or Release I/O occurs
// under this physical input mutex or the short admission mutex.
func (h Inputs) observeFresh(unitPaths []string) (returned error) {
	p := h.original
	if p == nil || len(unitPaths) == 0 || len(unitPaths) > 32 {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() { p.record(returned) }()
	if err := p.local(); err != nil {
		return err
	}
	r := p.request
	for _, derived := range r.reservedSiblingPaths() {
		for _, declared := range append(r.paths(), p.path) {
			if contains(derived, declared) || contains(declared, derived) {
				return ErrBinding
			}
		}
	}
	for _, path := range append([]string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory}, r.reservedSiblingPaths()...) {
		if _, err := p.parents(path); err != nil {
			return err
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	for _, directory := range unitPaths {
		if !validPath(directory) {
			return ErrBinding
		}
		// UnitPath includes absent optional roots. Retain every existing prefix
		// and require each currently absent suffix to stay absent in this round.
		prefix := directory
		for {
			_, err := os.Lstat(prefix)
			if err == nil {
				if _, err := p.parents(filepath.Join(prefix, "observation")); err != nil {
					return err
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || prefix == "/" {
				return errors.Join(ErrBinding, err)
			}
			prefix = filepath.Dir(prefix)
		}
		for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
			for _, suffix := range []string{".service", ".service.d", ".socket", ".socket.d"} {
				if _, err := os.Lstat(filepath.Join(directory, "ardents-"+role+suffix)); !errors.Is(err, os.ErrNotExist) {
					return errors.Join(ErrBinding, err)
				}
			}
		}
		for _, leaf := range []string{"ardents-.service.d", "service.d", "socket.d"} {
			if _, err := os.Lstat(filepath.Join(directory, leaf)); !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	for _, path := range []string{"/etc/passwd", "/etc/group"} {
		var raw []byte
		for _, file := range p.files {
			if file.path == path {
				raw = file.raw
				break
			}
		}
		if raw == nil {
			var err error
			raw, err = p.open(path, 1<<20, false)
			if err != nil {
				return err
			}
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if line != "" && strings.SplitN(line, ":", 2)[0] == "ardents-node" {
				return ErrBinding
			}
		}
	}
	return errors.Join(p.links(), p.local())
}
