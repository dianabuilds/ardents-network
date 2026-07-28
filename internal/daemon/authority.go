package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"time"

	domain "ardents/internal/authority"
	runtimeconfig "ardents/internal/config"
	diagapi "ardents/internal/diagnostics"
	apppolicy "ardents/internal/policy"
	"ardents/internal/storage"
)

func configureRealmAuthority(owners *Owners, config runtimeconfig.AuthorityConfig) *domain.FileStore {
	if owners == nil || !config.Enabled {
		return nil
	}
	policy := realmAuthorityPolicy{service: owners.RoutePolicy}
	unavailable := func() *domain.FileStore {
		owners.Authority = domain.New(domain.Config{Policy: policy})
		if owners.Node != nil {
			owners.Node.authority = owners.Authority
		}
		return nil
	}
	key, err := readRealmAuthorityStoreKey(config.StoreKeyFile)
	if err != nil {
		return unavailable()
	}
	defer clear(key)
	store, err := domain.OpenFileStore(context.Background(), config.StorePath, key)
	if err != nil {
		return unavailable()
	}
	signer, err := domain.NewFileSigner(config.SignerFile)
	if err != nil {
		_ = store.Close()
		return unavailable()
	}
	repository, err := domain.NewWORMFileCheckpointRepository(config.CheckpointRepositoryPath)
	if err != nil {
		_ = store.Close()
		return unavailable()
	}
	var audit domain.AuditSink
	if owners.Events != nil {
		audit = realmAuthorityAudit{events: owners.Events}
	}
	owners.Authority = domain.New(domain.Config{
		Store: store, Signer: signer, Repository: repository, Policy: policy,
		Audit: audit,
	})
	if owners.Node != nil {
		owners.Node.authority = owners.Authority
	}
	return store
}

func readRealmAuthorityStoreKey(path string) ([]byte, error) {
	raw, found, err := storage.ReadStrictPrivateFileBounded(path, 256)
	if err != nil || !found {
		return nil, domain.ErrUnavailable
	}
	encoded := bytes.TrimSpace(raw)
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	count, err := base64.StdEncoding.Strict().Decode(decoded, encoded)
	decoded = decoded[:count]
	canonical := make([]byte, base64.StdEncoding.EncodedLen(len(decoded)))
	base64.StdEncoding.Encode(canonical, decoded)
	valid := err == nil && len(decoded) == domain.AuthorityStoreKeyBytes && bytes.Equal(canonical, encoded)
	clear(canonical)
	clear(raw)
	if !valid {
		clear(decoded)
		return nil, domain.ErrCorruptState
	}
	return decoded, nil
}

type realmAuthorityAudit struct{ events diagapi.DurableEventWriter }

func (a realmAuthorityAudit) RecordAuthorityAudit(_ context.Context, record domain.AuditRecord) error {
	if a.events == nil {
		return domain.ErrUnavailable
	}
	eventType, message := "realm_authority_mutation", "Realm Authority mutation accepted"
	switch record.Action {
	case domain.ActionCreate:
		eventType, message = "realm_authority_genesis", "Realm Authority genesis accepted"
	case domain.ActionIssueDelivery:
		eventType, message = "realm_authority_delivery_issued", "Realm Authority delivery accepted"
	case domain.ActionAcknowledgeDelivery:
		eventType, message = "realm_authority_delivery_acknowledged", "Realm Authority delivery acknowledgement accepted"
	}
	_, err := a.events.RecordEventCommandDurable(diagapi.RecordEventCommand{
		Domain: "realm_authority", Type: eventType,
		Resource: "primary", Message: message,
		Payload: map[string]any{
			"version": record.Version, "audit_id": record.ID,
			"actor": record.Actor, "effective": record.Effective,
			"action": record.Action, "resource_kind": record.ResourceKind,
			"operation_id": record.OperationID, "outcome": record.Outcome,
			"audit_hash": record.Hash, "previous_hash": record.PreviousHash,
			"created_at": record.CreatedAt.Format(time.RFC3339),
		},
	})
	return err
}

type realmAuthorityPolicy struct{ service *apppolicy.Service }

func (p realmAuthorityPolicy) AdmitRealmGenesis(_ context.Context, _ domain.Command) error {
	if p.service == nil {
		return domain.ErrUnavailable
	}
	return p.service.AllowRealmAuthorityCreation()
}

func (p realmAuthorityPolicy) AdmitInitialGeneration(_ context.Context, _ domain.Command) error {
	if p.service == nil {
		return domain.ErrUnavailable
	}
	return p.service.AllowRealmChannelDelivery()
}

func (p realmAuthorityPolicy) AdmitChannelRotation(_ context.Context, _ domain.Command) error {
	if p.service == nil {
		return domain.ErrUnavailable
	}
	return p.service.AllowRealmChannelRotation()
}
