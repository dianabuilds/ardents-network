package infrastructure

import (
	"crypto/ed25519"
	"crypto/tls"
	"path/filepath"
	"reflect"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

// ServingPlan returns detached public declaration bytes under original input
// admission. It grants no State authority and contains no private credential.
func (s SourceInputs) ServingPlan() ([]byte, error) {
	p := s.original
	if p == nil {
		return nil, ErrInput
	}
	if err := p.begin(); err != nil {
		return nil, err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	if err := p.local(); err != nil {
		return nil, err
	}
	if len(p.files) < 2 || p.files[1].path != p.request.Plan {
		return nil, ErrInput
	}
	return append([]byte(nil), p.files[1].raw...), nil
}

func checkSourceRuntimePair(certificate, key []byte) error {
	pair, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		return ErrBinding
	}
	defer func() {
		if private, ok := pair.PrivateKey.(ed25519.PrivateKey); ok {
			clear(private)
		}
	}()
	return source.ValidateServingCredential(pair)
}

// The bounded replacement may move its unprivileged listener but retains the
// original Network/trust/credential/protection declaration. New actual runtime
// TLS and State observations are still required at independent startup.
func checkSourceForwardPlan(previous, candidate []byte) error {
	old, err := source.DecodeServingDeclaration(previous)
	if err != nil {
		return err
	}
	next, err := source.DecodeServingDeclaration(candidate)
	if err != nil {
		return err
	}
	next.Address = old.Address
	if !reflect.DeepEqual(old, next) {
		return ErrBinding
	}
	return nil
}

// Installation owns Source-specific matching against the original plan bytes.
// Source supplies only its one closed declaration codec; no State, Source or
// process-resource runtime is opened or granted accepting authority here.
func (r SourceRequest) checkPlan(raw []byte) error {
	plan, err := source.DecodeServingDeclaration(raw)
	if err != nil {
		return ErrInput
	}
	n := plan.Network
	if n == nil || n.NetworkID == [32]byte{} || n.ProfileAuthority == [32]byte{} || len(n.Authorities) == 0 || len(n.Authorities) > 16 || n.Threshold <= 0 || n.Threshold > len(n.Authorities) || len(plan.ClientPins) == 0 || len(plan.ClientPins) > 3 {
		return ErrInput
	}
	if n.Root != r.StateRoot || plan.ParticipationRoot != r.ParticipationRoot || n.ClockObservationFile != r.Clock || plan.Address != r.Address || plan.ProcessProfile != r.Profile ||
		plan.Certificate != filepath.Join(r.CredentialDirectory, "certificate.pem") || plan.PrivateKey != filepath.Join(r.CredentialDirectory, "private-key.pem") || plan.ClientRoot != filepath.Join(r.CredentialDirectory, "client-ca.pem") {
		return ErrBinding
	}
	return nil
}
