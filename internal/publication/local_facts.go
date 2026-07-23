package publication

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"time"

	"ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/records"
	identityapi "ardents/internal/identity"
	identityprincipal "ardents/internal/identity/principal"
)

type LocalServiceSpec struct {
	ID         string
	Type       string
	WorkloadID string
	Mode       string
	Endpoints  []string
}

func PublishLocalNode(disco *discovery.Service, id identityapi.Summary, key ed25519.PrivateKey, endpoints []string) error {
	record, err := localNodeRecord(id, key, endpoints)
	if err != nil {
		return err
	}
	return applyLocalRecord(disco, record)
}

func WithdrawLocalNode(disco *discovery.Service, id identityapi.Summary, key ed25519.PrivateKey) error {
	record, err := localNodeRecord(id, key, nil)
	if err != nil {
		return err
	}
	return applyLocalRecord(disco, record)
}

func PublishLocalService(disco *discovery.Service, id identityapi.Summary, key ed25519.PrivateKey, spec LocalServiceSpec) error {
	record, err := localServiceRecord(id, key, spec)
	if err != nil {
		return err
	}
	return applyLocalRecord(disco, record)
}

func WithdrawAllLocalServices(disco *discovery.Service, id identityapi.Summary, key ed25519.PrivateKey) error {
	if disco == nil {
		return nil
	}
	for _, item := range disco.Entries() {
		if item.Source != discoverysource.Local || item.Record.Kind() != discoverysource.KindService {
			continue
		}
		if err := PublishLocalService(disco, id, key, LocalServiceSpec{
			ID:         item.Record.RecordID(),
			Type:       item.Record.ServiceType(),
			WorkloadID: item.Record.WorkloadID(),
			Mode:       item.Record.ServiceMode(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyLocalRecord(disco *discovery.Service, record discovery.Record) error {
	if disco == nil {
		return errors.New("discovery service is unavailable")
	}
	result, err := disco.Import(record, discoverysource.Local)
	if err != nil {
		return err
	}
	if !result.Applied {
		return errors.New(result.Reason)
	}
	return nil
}

func localNodeRecord(id identityapi.Summary, key ed25519.PrivateKey, endpoints []string) (discovery.Record, error) {
	principal, err := identityprincipal.Parse(id.Principal)
	if err != nil {
		return discovery.Record{}, errors.New("local Node Principal is invalid")
	}
	record := discovery.Record{
		Version: discoverysource.Version,
		Node: &discoverysource.NodeFacts{
			Principal: principal,
			PublicKey: id.PublicKey,
			Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(discovery.LocalRecordTTL),
	}
	return signLocalRecord(record, key)
}

func localServiceRecord(id identityapi.Summary, key ed25519.PrivateKey, spec LocalServiceSpec) (discovery.Record, error) {
	principal, err := identityprincipal.Parse(id.Principal)
	if err != nil {
		return discovery.Record{}, errors.New("local Node Principal is invalid")
	}
	record := discovery.Record{
		Version: discoverysource.Version,
		Service: &discoverysource.ServiceFacts{
			ID:            discoverysource.ServiceID(spec.ID),
			Type:          spec.Type,
			NodePrincipal: principal,
			Workload:      discoverysource.WorkloadID(spec.WorkloadID),
			Mode:          spec.Mode,
			PublicKey:     id.PublicKey,
			Endpoints:     append([]string(nil), spec.Endpoints...),
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(discovery.LocalRecordTTL),
	}
	return signLocalRecord(record, key)
}

func signLocalRecord(record discovery.Record, key ed25519.PrivateKey) (discovery.Record, error) {
	payload, err := discovery.Canonical(record)
	if err != nil {
		return discovery.Record{}, err
	}
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return record, nil
}
