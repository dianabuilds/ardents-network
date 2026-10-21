package infrastructure

import (
	"crypto/ed25519"
	"crypto/tls"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

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
