package descriptor

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/request"
)

// Inputs owns one original Descriptor declaration and its protected delivery files.
// Copies retain the same caller and Close result; detached declarations grant
// no creation, Release, runtime or startup permission.
type Inputs struct{ original *inputs }

type inputs struct {
	mu      sync.Mutex
	uses    sync.WaitGroup
	once    sync.Once
	ctx     context.Context
	closed  bool
	failure error
	path    string
	request Request
	files   request.Files
}

var errInputsClosed = errors.New("descriptor installation: original inputs closed")

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
		// Join original borrowers, then reobserve delivery before physical release.
		// Neither a late mutation nor a Close failure may become clean output.
		physical := errors.Join(p.files.Check(), p.files.Close())
		p.mu.Lock()
		p.failure = errors.Join(p.failure, p.ctx.Err(), physical)
		p.mu.Unlock()
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failure
}
