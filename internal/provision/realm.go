package provision

import (
	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/internal/storage"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	authorityVersion = "ardents.local-realm/v2"
	nodeVersion      = "ardents.local-realm-node/v2"
	grantLifetime    = 30 * 24 * time.Hour
	refreshBefore    = 24 * time.Hour
)

type Authority struct {
	state authorityState
	key   ed25519.PrivateKey
	path  string
}

type NodeOptions struct {
	DataDir   string
	SecretDir string
	Clock     func() time.Time
}

type NodeProvision struct {
	Subject              string
	Issuer               string
	IssuerPublic         ed25519.PublicKey
	DiscoveryRef         identityapi.CapabilityRef
	DataRef              identityapi.CapabilityRef
	StoreKeyPath         string
	ReplayKeyPath        string
	CapabilityStore      string
	DiscoveryReplay      string
	DataReplay           string
	ApplicationExpiresAt time.Time
}

type authorityState struct {
	Version       string               `json:"version"`
	IssuerPrivate string               `json:"issuer_private"`
	Discovery     channelState         `json:"discovery"`
	Data          channelState         `json:"data"`
	Members       map[string]nodeState `json:"members,omitempty"`
}

type channelState struct {
	ID         string `json:"id"`
	Secret     string `json:"secret"`
	Generation uint32 `json:"generation"`
}

type nodeState struct {
	Version   string     `json:"version"`
	Subject   string     `json:"subject"`
	Issuer    string     `json:"issuer"`
	Discovery grantState `json:"discovery"`
	Data      grantState `json:"data"`
}

type grantState struct {
	ID        string    `json:"id"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

func OpenOrCreate(dir string) (*Authority, error) {
	if err := ensurePrivateDir(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "authority.json")
	state, err := loadAuthority(path)
	if errors.Is(err, os.ErrNotExist) {
		state, err = newAuthorityState(rand.Reader)
		if err == nil {
			err = writePrivateJSON(path, state)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("local realm authority is unavailable or invalid")
	}
	key, err := validateAuthority(state)
	if err != nil {
		return nil, fmt.Errorf("local realm authority is unavailable or invalid")
	}
	if state.Members == nil {
		state.Members = make(map[string]nodeState)
	}
	return &Authority{state: state, key: key, path: path}, nil
}

func (a *Authority) ProvisionNode(options NodeOptions, admission identityapi.CapabilityAdmission) (NodeProvision, error) {
	if admission == nil {
		return NodeProvision{}, fmt.Errorf("capability admission is required")
	}
	if err := ensurePrivateDir(options.DataDir); err != nil {
		return NodeProvision{}, err
	}
	if err := ensurePrivateDir(options.SecretDir); err != nil {
		return NodeProvision{}, err
	}
	state := identityapi.NewStoreInDir(options.DataDir)
	if err := state.Load(); err != nil {
		return NodeProvision{}, fmt.Errorf("load canonical node identity state: %w", err)
	}
	summary, _, err := identityapi.NewService().EnsureNode(
		state, identitykeyring.NewKeyStoreInDir(options.DataDir),
	)
	if err != nil {
		return NodeProvision{}, fmt.Errorf("initialize canonical node identity: %w", err)
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	return a.provisionSubject(options, summary.Principal, admission, clock().UTC().Truncate(time.Second))
}

func (a *Authority) provisionSubject(options NodeOptions, subject string, admission identityapi.CapabilityAdmission, now time.Time) (NodeProvision, error) {
	issuerPublic := a.key.Public().(ed25519.PublicKey)
	issuerID, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	if err != nil {
		return NodeProvision{}, fmt.Errorf("derive local realm issuer Principal")
	}
	issuer := issuerID.String()
	trustedIssuers, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: issuer, PublicKey: issuerPublic,
		Purposes: []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	if err != nil {
		return NodeProvision{}, fmt.Errorf("build local realm trust registry")
	}
	nodeStorage, err := prepareNodeStorage(options)
	if err != nil {
		return NodeProvision{}, err
	}
	service, err := identitycapability.NewService(nodeStorage.capabilityStore, nodeStorage.storeKey, subject,
		trustedIssuers, admission, func() time.Time { return now })
	if err != nil {
		return NodeProvision{}, fmt.Errorf("open protected capability store: %w", err)
	}
	record, err := a.loadOrCreateNodeState(nodeStorage.recordPath, subject, issuer, now)
	if err != nil {
		return NodeProvision{}, err
	}
	discoveryRef, err := a.importGrant(service, record.Discovery, subject, issuer, a.state.Discovery, identityapi.CapabilityRealmDiscovery)
	if err != nil {
		return NodeProvision{}, err
	}
	dataRef, err := a.importGrant(service, record.Data, subject, issuer, a.state.Data, identityapi.CapabilityDataExchange)
	if err != nil {
		return NodeProvision{}, err
	}
	if err := a.importSenderGrants(service, subject); err != nil {
		return NodeProvision{}, err
	}
	return NodeProvision{
		Subject: subject, Issuer: issuer, IssuerPublic: append(ed25519.PublicKey(nil), issuerPublic...),
		DiscoveryRef: discoveryRef, DataRef: dataRef, StoreKeyPath: nodeStorage.storeKeyPath,
		ReplayKeyPath: nodeStorage.replayKeyPath, CapabilityStore: nodeStorage.capabilityStore,
		DiscoveryReplay: nodeStorage.discoveryReplay, DataReplay: nodeStorage.dataReplay,
		ApplicationExpiresAt: earliestTime(record.Discovery.NotAfter, record.Data.NotAfter),
	}, nil
}

func earliestTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

func (a *Authority) loadOrCreateNodeState(path, subject, issuer string, now time.Time) (nodeState, error) {
	var state nodeState
	err := readPrivateJSON(path, &state)
	if err == nil {
		if state.Version != nodeVersion || state.Subject != subject || state.Issuer != issuer {
			return nodeState{}, fmt.Errorf("local realm node state does not match canonical identity")
		}
		if state.Discovery.NotAfter.After(now.Add(refreshBefore)) && state.Data.NotAfter.After(now.Add(refreshBefore)) {
			return a.registerMember(path, state)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nodeState{}, fmt.Errorf("local realm node state is unavailable or invalid")
	}
	discovery, err := newGrantState(now)
	if err != nil {
		return nodeState{}, err
	}
	data, err := newGrantState(now)
	if err != nil {
		return nodeState{}, err
	}
	state = nodeState{Version: nodeVersion, Subject: subject, Issuer: issuer,
		Discovery: discovery, Data: data}
	return a.registerMember(path, state)
}

func (a *Authority) registerMember(path string, state nodeState) (nodeState, error) {
	if current, ok := a.state.Members[state.Subject]; ok {
		if current.Issuer != state.Issuer || current.Discovery.ID != state.Discovery.ID || current.Data.ID != state.Data.ID {
			return nodeState{}, fmt.Errorf("local realm member state conflicts with authority")
		}
		state = current
	} else {
		a.state.Members[state.Subject] = state
		if err := writePrivateJSON(a.path, a.state); err != nil {
			return nodeState{}, err
		}
	}
	if err := writePrivateJSON(path, state); err != nil {
		return nodeState{}, err
	}
	return state, nil
}

func (a *Authority) importGrant(service *identitycapability.Service, record grantState, subject, issuer string, channel channelState, scope identityapi.CapabilityScope) (identityapi.CapabilityRef, error) {
	grant, err := a.signedGrant(record, subject, issuer, channel, scope)
	if err != nil {
		return "", err
	}
	ref, err := service.ImportGrant(grant)
	if err != nil {
		return "", fmt.Errorf("import local realm grant: %w", err)
	}
	return ref, nil
}

func (a *Authority) importSenderGrants(service *identitycapability.Service, localSubject string) error {
	for subject, member := range a.state.Members {
		if subject == localSubject {
			continue
		}
		for _, item := range []struct {
			record  grantState
			channel channelState
			scope   identityapi.CapabilityScope
		}{{member.Discovery, a.state.Discovery, identityapi.CapabilityRealmDiscovery},
			{member.Data, a.state.Data, identityapi.CapabilityDataExchange}} {
			grant, err := a.signedGrant(item.record, subject, member.Issuer, item.channel, item.scope)
			if err != nil {
				return err
			}
			if err := service.ImportSenderGrant(grant); err != nil {
				return fmt.Errorf("import local realm sender grant: %w", err)
			}
		}
	}
	return nil
}

func (a *Authority) signedGrant(record grantState, subject, issuer string, channel channelState, scope identityapi.CapabilityScope) (identityapi.CapabilityGrant, error) {
	channelID, secret, grantID, err := decodeGrantMaterial(channel, record)
	if err != nil {
		return identityapi.CapabilityGrant{}, fmt.Errorf("local realm grant state is unavailable or invalid")
	}
	grant, err := identitycapability.SignGrant(identityapi.CapabilityGrant{
		Version: 1, ChannelID: channelID, Generation: channel.Generation, Secret: secret,
		GrantID: grantID, IssuerPrincipal: issuer, SubjectPrincipal: subject,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe | identityapi.CapabilityStoreFetch,
		Scope:       scope, NotBefore: record.NotBefore, NotAfter: record.NotAfter,
	}, a.key)
	if err != nil {
		return identityapi.CapabilityGrant{}, fmt.Errorf("sign local realm grant: %w", err)
	}
	return grant, nil
}

func newGrantState(now time.Time) (grantState, error) {
	id := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return grantState{}, err
	}
	return grantState{ID: base64.StdEncoding.EncodeToString(id),
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(grantLifetime)}, nil
}

func loadOrCreateKey(path, protectedState string) ([]byte, error) {
	raw, err := readPrivate(path)
	if err == nil {
		key, decodeErr := base64.StdEncoding.DecodeString(string(raw))
		if decodeErr != nil || len(key) != 32 {
			return nil, fmt.Errorf("protected local realm key is unavailable or invalid")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) || fileExists(protectedState) {
		return nil, fmt.Errorf("protected local realm key is unavailable or invalid")
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := storage.AtomicWritePrivateFile(path, []byte(base64.StdEncoding.EncodeToString(key))); err != nil {
		return nil, err
	}
	return key, nil
}
