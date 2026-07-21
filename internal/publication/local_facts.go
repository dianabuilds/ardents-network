package publication

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"time"

	"ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/records"
	identityapi "ardents/internal/identity"
)

type LocalServiceSpec struct {
	ID        string
	Type      string
	Owner     string
	Mode      string
	Endpoints []string
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
		if item.Source != discoverysource.Local || item.Record.Kind != "service" {
			continue
		}
		if err := PublishLocalService(disco, id, key, LocalServiceSpec{
			ID:    item.Record.ID,
			Type:  item.Record.Service,
			Owner: item.Record.Owner,
			Mode:  item.Record.Mode,
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
	record := discovery.Record{
		ID:        id.Principal + ":node",
		Kind:      "node",
		Subject:   id.Principal,
		Node:      id.Principal,
		Device:    id.Device,
		PublicKey: id.PublicKey,
		Endpoints: append([]string(nil), endpoints...),
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(discovery.LocalRecordTTL),
	}
	return signLocalRecord(record, key)
}

func localServiceRecord(id identityapi.Summary, key ed25519.PrivateKey, spec LocalServiceSpec) (discovery.Record, error) {
	record := discovery.Record{
		ID:        spec.ID,
		Kind:      "service",
		Subject:   spec.ID,
		Node:      id.Principal,
		Device:    id.Device,
		Owner:     spec.Owner,
		Service:   spec.Type,
		Mode:      spec.Mode,
		PublicKey: id.PublicKey,
		Endpoints: append([]string(nil), spec.Endpoints...),
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
