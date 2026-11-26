//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func installedIdentity(encoded string) ([32]byte, error) {
	value, ok := canonicaldigest.Decode(encoded)
	if !ok {
		return value, errors.New("installed identity is unavailable")
	}
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

// NetworkConfig projects already-bound Installation declarations. Original
// Startup/preparation checks surround every native input; State independently
// authenticates signed current facts after opening.
func NetworkConfig(plan HeadlessPlan, declared SourcePlan, check func() error) (state.Config, error) {
	if check == nil || len(declared.Sources) != 2 {
		return state.Config{}, errors.New("invalid bound Source declaration")
	}
	config, err := plan.Network()
	if err != nil {
		return state.Config{}, err
	}
	config.LocalRoleStateRoot = plan.LocalRoleStateRoot
	config.ClockObservation, err = time.Parse(time.RFC3339, declared.ClockObservedAt)
	if err != nil {
		return state.Config{}, err
	}
	return installedAcquisitionConfig(config, declared, check)
}

// Network returns detached trust configuration only, without native input or
// State effects. The offline Source producer consumes this same projection.
func (plan HeadlessPlan) Network() (state.Config, error) {
	id, err := installedIdentity(plan.NetworkID)
	if err != nil || id == [32]byte{} {
		return state.Config{}, errors.New("invalid network identity")
	}
	profile, err := installedIdentity(plan.ClosedProfileAuthority)
	if err != nil || profile == [32]byte{} {
		return state.Config{}, errors.New("invalid profile authority")
	}
	if !pathgrammar.Strict(plan.NetworkStateRoot) || !pathgrammar.Strict(plan.TimeConfidenceFile) || len(plan.NetworkAuthorities) == 0 || len(plan.NetworkAuthorities) > 16 {
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

func installedAcquisitionConfig(config state.Config, declared SourcePlan, check func() error) (state.Config, error) {
	var err error
	config.Source.OrderSeed, err = installedIdentity(declared.OrderSeed)
	if err != nil {
		return state.Config{}, err
	}
	config.Source.MaterialIndex = declared.MaterializationIndex
	config.AutomaticRefreshInterval = time.Duration(declared.RefreshIntervalMS) * time.Millisecond
	read := func(path string, maximum int64) ([]byte, error) {
		if err := check(); err != nil {
			return nil, err
		}
		raw, err := readBounded(path, maximum)
		if checked := check(); checked != nil {
			clear(raw)
			return nil, errors.Join(err, checked)
		}
		return raw, err
	}
	certificate, err := read(declared.ClientCertificate, 64<<10)
	if err != nil {
		return state.Config{}, err
	}
	defer clear(certificate)
	key, err := read(declared.ClientKey, 16<<10)
	if err != nil {
		return state.Config{}, err
	}
	defer clear(key)
	config.Source.ClientCertificate, err = tls.X509KeyPair(certificate, key)
	if err != nil {
		return state.Config{}, err
	}
	for index, member := range declared.Sources {
		identity, err := installedIdentity(member.Identity)
		if err != nil {
			return state.Config{}, err
		}
		leaf, err := installedIdentity(member.LeafKeyDigest)
		if err != nil {
			return state.Config{}, err
		}
		root, err := read(member.RootCA, 64<<10)
		if err != nil {
			return state.Config{}, err
		}
		config.Source.Sources[index] = source.Source{Address: member.Address, ServerName: member.ServerName, Identity: identity, Family: member.Family, EndpointHandle: member.EndpointHandle, RootPEM: root, LeafKeyDigest: leaf}
	}
	return config, check()
}

// This bounded reader observes the opened object, so a substituted FIFO cannot
// block opening. Startup supplies original custody; this reader supplies none.
func readBounded(path string, maximum int64) (raw []byte, result error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, file.Close())
		if result != nil {
			clear(raw)
			raw = nil
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(errors.New("input unavailable"), err)
	}
	raw, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(raw)) > maximum {
		clear(raw)
		return nil, errors.Join(errors.New("input unavailable"), err)
	}
	return raw, nil
}
