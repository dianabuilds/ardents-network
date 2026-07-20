package data

import (
	"path/filepath"
	"sync"
	"time"

	availabilitypkg "ardents/internal/data/availability"
	model "ardents/internal/data/model"
	"ardents/internal/data/placement"
	statepkg "ardents/internal/data/state"
)

const defaultMaxReplicaRetentionBytes int64 = 1 << 30

type Service struct {
	mu           sync.Mutex
	path         string
	dir          string
	cfg          Config
	state        string
	localNodeID  string
	retention    RetentionAuthorizer
	objects      statepkg.ObjectStore
	blobs        statepkg.BlobStore
	sources      statepkg.SourceLedger
	transfers    statepkg.TransferLedger
	manifests    statepkg.ManifestStore
	placement    *placement.Receiver
	availability availabilitypkg.State
}

func New(path string) *Service {
	return NewWithConfig(path, Config{})
}

func NewWithConfig(path string, cfg Config) *Service {
	dir := filepath.Dir(path)
	service := &Service{
		path:         path,
		dir:          dir,
		cfg:          cfg,
		state:        "new",
		objects:      statepkg.NewObjectStore(),
		blobs:        statepkg.NewBlobStore(),
		sources:      statepkg.NewSourceLedger(),
		transfers:    statepkg.NewTransferLedger(),
		manifests:    statepkg.NewManifestStore(),
		availability: availabilitypkg.NewState(),
	}
	maxReplicaBytes := cfg.MaxReplicaRetentionBytes
	if maxReplicaBytes <= 0 {
		maxReplicaBytes = cfg.MaxRelayRetentionBytes
	}
	if maxReplicaBytes <= 0 {
		maxReplicaBytes = defaultMaxReplicaRetentionBytes
	}
	service.placement = placement.NewReceiver(placement.ReceiverConfig{
		MaxBytes: maxReplicaBytes,
		Store: func(blob model.Blob, ciphertext []byte, expiresAt time.Time) error {
			_, err := service.RetainRelayBlob(blob, ciphertext, expiresAt)
			return err
		},
	})
	return service
}

func NewInDir(dir string) *Service {
	return New(statepkg.PathInDir(dir))
}

func NewInDirWithConfig(dir string, cfg Config) *Service {
	return NewWithConfig(statepkg.PathInDir(dir), cfg)
}
