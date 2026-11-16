package endpoint

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func installedIdentity(encoded string) ([32]byte, error) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) || hex.EncodeToString(raw) != encoded {
		return value, errors.New("installed identity is unavailable")
	}
	copy(value[:], raw)
	return value, nil
}

func installedLocalConfig(plan HeadlessPlan) (execution.Config, error) {
	id, err := installedIdentity(plan.BrokerID)
	if err != nil {
		return execution.Config{}, err
	}
	reader, err := installedIdentity(plan.ConnectionPrincipal)
	if err != nil {
		return execution.Config{}, err
	}
	config := execution.Config{ID: id, Grants: []execution.Grant{{Principal: reader, Surface: execution.Connection}}}
	if plan.Role == "" {
		publisher, err := installedIdentity(plan.AdministrationPrincipal)
		if err != nil {
			return execution.Config{}, err
		}
		config.Grants = append(config.Grants, execution.Grant{Principal: publisher, Surface: execution.Administration})
	}
	return config, nil
}

// Network returns detached trust configuration only, without native input or
// State effects. Paths retain the selected Linux declaration's POSIX grammar;
// parsing them on another OS does not qualify that OS for installed execution.
func (plan HeadlessPlan) Network() (state.Config, error) {
	id, err := installedIdentity(plan.NetworkID)
	if err != nil || id == [32]byte{} {
		return state.Config{}, errors.New("invalid network identity")
	}
	profile, err := installedIdentity(plan.ClosedProfileAuthority)
	if err != nil || profile == [32]byte{} {
		return state.Config{}, errors.New("invalid profile authority")
	}
	if !path.IsAbs(plan.NetworkStateRoot) || path.Clean(plan.NetworkStateRoot) != plan.NetworkStateRoot || !path.IsAbs(plan.TimeConfidenceFile) || path.Clean(plan.TimeConfidenceFile) != plan.TimeConfidenceFile || len(plan.NetworkAuthorities) == 0 || len(plan.NetworkAuthorities) > 16 {
		return state.Config{}, errors.New("invalid network declaration")
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(plan.NetworkAuthorities))
	for _, encoded := range plan.NetworkAuthorities {
		key, err := installedIdentity(encoded)
		if err != nil || key == [32]byte{} {
			return state.Config{}, errors.New("invalid authority pin")
		}
		digest := sha256.Sum256(key[:])
		if _, exists := authorities[digest]; exists {
			return state.Config{}, errors.New("duplicate authority pin")
		}
		authorities[digest] = append(ed25519.PublicKey(nil), key[:]...)
	}
	return state.Config{Root: plan.NetworkStateRoot, NetworkID: id, Authorities: authorities, Threshold: plan.NetworkThreshold, ClosedProfileAuthority: append(ed25519.PublicKey(nil), profile[:]...), AcceptedProfile: "ardents-route-v3", Clock: time.Now, ClockObservationFile: plan.TimeConfidenceFile}, nil
}
