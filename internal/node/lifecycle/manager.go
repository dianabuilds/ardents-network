package lifecycle

import (
	"context"
	"crypto/ed25519"
	"time"

	dataapi "ardents/internal/data/api"
	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	noderecovery "ardents/internal/node/recovery"
	workloadapi "ardents/internal/workload/api"
	domainworkload "ardents/internal/workload/workload"
)

type AuthorityCoordinator interface {
	LoadData() error
	LoadWorkloads() error
	SetLocalDataNodeID(string)
	SeedWorkloadsAndReconcileLocked(context.Context, []domainworkload.Spec) error
	ShutdownWorkloadsLocked(context.Context) error
	SyncObservedWorkloadsLocked(context.Context) error
	SyncDiscoveryTrustDiagnosticsLocked()
	StartBlobExchangeLocked(context.Context) error
	ResolveRecordLocked(string, string) (discoveryapi.DiscoveryResult, error)
	ResolveServiceLocked(string) (discoveryapi.ServiceResult, error)
	ListRecordsLocked() ([]discoveryapi.DiscoveryRecord, error)
	ImportRecordLocked(discoveryapi.DiscoveryRecord) (discoveryapi.RecordImportResult, error)
	ListWorkloadsLocked() ([]workloadapi.WorkloadStatusSnapshot, error)
	GetWorkloadLocked(string) (workloadapi.WorkloadStatusSnapshot, error)
	RegisterWorkloadLocked(context.Context, workloadapi.WorkloadSpecSnapshot) error
	StartWorkloadLocked(context.Context, string) error
	StopWorkloadLocked(context.Context, string) error
	PublishObjectLocked(dataapi.ObjectSnapshot) (dataapi.ObjectSnapshot, error)
	GetObjectLocked(string) (dataapi.ObjectSnapshot, error)
	ListObjectsLocked() ([]dataapi.ObjectSnapshot, error)
	PublishBlobLocked(dataapi.BlobSnapshot) (dataapi.BlobSnapshot, error)
	FetchBlobLocked(context.Context, string) (dataapi.BlobSnapshot, error)
	GetBlobLocked(string) (dataapi.BlobSnapshot, error)
	ListBlobsLocked() ([]dataapi.BlobSnapshot, error)
	PublishManifestLocked(dataapi.ManifestSnapshot) (dataapi.ManifestSnapshot, error)
	GetManifestLocked(string) (dataapi.ManifestSnapshot, error)
	ListManifestsLocked() ([]dataapi.ManifestSnapshot, error)
	RetainBlobLocked(string, time.Time) (dataapi.BlobSnapshot, error)
	PinBlobLocked(string) (dataapi.BlobSnapshot, error)
	DropBlobLocked(string) (dataapi.BlobSnapshot, error)
	DataInventoryLocked() dataapi.DataInventorySnapshot
}

type PublicationCoordinator interface {
	RefreshNetworkPublicationLocked(context.Context) error
	WithdrawNetworkPublicationLocked(context.Context) error
}

type RuntimeCoordinator interface {
	StartLocked(context.Context) error
	StopLocked(context.Context) error
	SyncObservedTruthLocked()
	RefreshDiscoveryPublicationLocked(context.Context)
	FailLocked(code, domain, summary, detail, impact, recovery string)
}

type Manager struct {
	cfgName       string
	bootSources   []string
	workloadSpecs []domainworkload.Spec
	life          *Machine
	diag          *diagnostics.Recorder
	state         *noderecovery.Store
	keys          identityapi.KeyStore
	boot          *noderecovery.BootStatus
	ident         identityapi.Service
	trust         *discovery.TrustEvaluator
	disco         *discovery.Service
	trans         transport.Service
	privacy       *networkprivacy.Channel
	authority     AuthorityCoordinator
	publication   PublicationCoordinator
	getPrivate    func() ed25519.PrivateKey
	setPrivate    func(ed25519.PrivateKey)
	publish       func(string, map[string]any)
}

func NewRuntimeCoordinator(
	cfgName string, bootSources []string, workloadSpecs []domainworkload.Spec,
	life *Machine, diag *diagnostics.Recorder,
	state *noderecovery.Store, keys identityapi.KeyStore, boot *noderecovery.BootStatus,
	ident identityapi.Service, trustSvc *discovery.TrustEvaluator,
	disco *discovery.Service, trans transport.Service,
	authorityCtl AuthorityCoordinator, publicationMgr PublicationCoordinator,
	getPrivate func() ed25519.PrivateKey,
	setPrivate func(ed25519.PrivateKey),
	publish func(string, map[string]any),
	privateChannels ...*networkprivacy.Channel,
) *Manager {
	var privacyChannel *networkprivacy.Channel
	if len(privateChannels) > 0 {
		privacyChannel = privateChannels[0]
	}
	return &Manager{
		cfgName:       cfgName,
		bootSources:   append([]string(nil), bootSources...),
		workloadSpecs: append([]domainworkload.Spec(nil), workloadSpecs...),
		life:          life,
		diag:          diag,
		state:         state,
		keys:          keys,
		boot:          boot,
		ident:         ident,
		trust:         trustSvc,
		disco:         disco,
		trans:         trans,
		privacy:       privacyChannel,
		authority:     authorityCtl,
		publication:   publicationMgr,
		getPrivate:    getPrivate,
		setPrivate:    setPrivate,
		publish:       publish,
	}
}
