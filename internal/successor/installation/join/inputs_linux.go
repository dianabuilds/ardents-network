package join

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/request"
)

// Inputs owns one original JOIN declaration and its protected delivery files.
// Copies retain the same caller and Close result; detached declarations grant
// no creation, Release, runtime or startup permission.
type Inputs struct{ original *inputs }

type inputs struct {
	mu         sync.Mutex
	uses       sync.WaitGroup
	once       sync.Once
	ctx        context.Context
	closed     bool
	installing bool
	inspected  bool
	failure    error
	path       string
	request    Request
	files      request.Files
}

var errInputsClosed = errors.New("JOIN installation: original inputs closed")

// ReadInputs retains direct existing root-controlled request/plan/TLS inputs.
// The clock remains live under its original root-writer policy. No semantic
// history, manager, account, destination or Release verifier is opened.
func ReadInputs(ctx context.Context, path string) (result Inputs, returned error) {
	if ctx == nil || !validPath(path) {
		return result, ErrInput
	}
	files, err := request.OpenFiles(ctx)
	if err != nil {
		return result, err
	}
	p := &inputs{ctx: ctx, path: path, files: files}
	result.original = p
	defer func() {
		if returned != nil {
			p.record(returned)
			returned = result.Close()
			result = Inputs{}
		}
	}()
	raw, err := files.Read(path, 16<<10, true)
	if err != nil {
		return result, err
	}
	p.request, err = DecodeRequest(raw)
	clear(raw)
	if err != nil {
		return result, err
	}
	for _, declared := range p.request.ownedPaths() {
		if contains(path, declared) || contains(declared, path) {
			return result, ErrBinding
		}
	}
	raw, err = files.Read(p.request.Plan, 16<<10, true)
	if err != nil {
		return result, err
	}
	plan, err := DecodePlan(raw)
	clear(raw)
	if err != nil {
		return result, err
	}
	if err := p.request.MatchPlan(plan); err != nil {
		return result, err
	}
	certificate, err := files.Read(p.request.Certificate, 8192, false)
	if err != nil {
		return result, err
	}
	defer clear(certificate)
	private, err := files.Read(p.request.PrivateKey, 8192, true)
	if err != nil {
		return result, err
	}
	defer clear(private)
	pair, err := tls.X509KeyPair(certificate, private)
	if err != nil {
		return result, err
	}
	key, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if ok {
		defer clear(key)
	}
	pair.PrivateKey = nil
	if !ok || len(key) != ed25519.PrivateKeySize {
		return result, ErrBinding
	}
	if err := files.RetainParents(p.request.Clock); err != nil {
		return result, err
	}
	return result, result.Check()
}

func (r Request) ownedPaths() []string {
	return append(r.paths(), r.reservedSiblingPaths()...)
}

func (p *inputs) begin() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return p.failure
	}
	if err := p.ctx.Err(); err != nil {
		if !p.closed {
			p.failure = err
		}
		return err
	}
	if p.closed {
		return errInputsClosed
	}
	p.uses.Add(1)
	return nil
}

func (p *inputs) record(err error) {
	if err == nil || err == errInputsClosed {
		return
	}
	p.mu.Lock()
	if p.failure == nil {
		p.failure = err
	}
	p.mu.Unlock()
}

func (p *inputs) local() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return p.failure
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if p.closed {
		return errInputsClosed
	}
	return nil
}

func (h Inputs) Check() (returned error) {
	p := h.original
	if p == nil {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	defer func() { p.record(returned) }()
	if err := p.files.Check(); err != nil {
		return err
	}
	if err := p.files.CheckClock(p.request.Clock); err != nil {
		return err
	}
	return p.local()
}

func (h Inputs) Close() error {
	p := h.original
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		p.uses.Wait()
		physical := p.files.Close()
		p.mu.Lock()
		p.failure = errors.Join(p.failure, p.ctx.Err(), physical)
		p.mu.Unlock()
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failure
}

// IndependentPaths performs no Release opening: canonical root composition
// calls it before the mutable verifier can create or advance its own history.
func (h Inputs) IndependentPaths(distribution, history string) error {
	p := h.original
	if p == nil || !validPath(distribution) || !validPath(history) {
		return ErrInput
	}
	if contains(distribution, history) || contains(history, distribution) {
		return ErrBinding
	}
	for _, path := range append(p.request.ownedPaths(), p.path) {
		if contains(distribution, path) || contains(path, distribution) || contains(history, path) || contains(path, history) {
			return ErrBinding
		}
	}
	return h.Check()
}

type deliveredInput struct{ raw []byte }

func (p *inputs) readDelivery() (files []deliveredInput, returned error) {
	defer func() {
		if returned != nil {
			for _, file := range files {
				clear(file.raw)
			}
			files = nil
		}
	}()
	for _, declared := range []struct {
		path    string
		maximum int64
		private bool
	}{{p.path, 16 << 10, true}, {p.request.Plan, 16 << 10, true}, {p.request.Certificate, 8192, false}, {p.request.PrivateKey, 8192, true}} {
		raw, err := p.files.Read(declared.path, declared.maximum, declared.private)
		if err != nil {
			return files, err
		}
		files = append(files, deliveredInput{raw})
	}
	return files, nil
}

// CheckRecoveryPaths retains the original public selection declaration before
// canonical composition opens a mutable Release verifier. It supplies no cut,
// completed history or mutation right; recovery independently reopens them.
func (h Inputs) CheckRecoveryPaths(bindingPath, distribution, history string) (returned error) {
	p := h.original
	if p == nil || bindingPath != filepath.Join(p.request.InstallationRoot, "binding.json") {
		return ErrInput
	}
	if err := h.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	defer func() { p.record(returned) }()
	raw, err := p.files.Read(bindingPath, 32<<10, false)
	if err != nil {
		return err
	}
	defer clear(raw)
	selected, err := decodeSelection(raw)
	if err != nil || selected.replacement == nil || selected.binding.History != history || selected.binding.Request != p.request {
		return errors.Join(ErrBinding, err)
	}
	return errors.Join(p.files.Check(), p.local())
}
