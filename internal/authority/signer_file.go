package authority

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"

	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
)

type FileSigner struct{ path string }

const maxSignerFileBytes int64 = 4096

type fileSignerRecord struct {
	Version    uint32 `json:"version"`
	Principal  string `json:"principal"`
	PrivateKey string `json:"private_key"`
}

func NewFileSigner(path string) (*FileSigner, error) {
	if path == "" {
		return nil, ErrInvalidArgument
	}
	return &FileSigner{path: path}, nil
}

func (s *FileSigner) Principal(ctx context.Context) (string, error) {
	record, private, err := s.load(ctx)
	clear(private)
	if err != nil {
		return "", err
	}
	return record.Principal, nil
}

func (s *FileSigner) PublicKey(ctx context.Context) (ed25519.PublicKey, error) {
	_, private, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	return append(ed25519.PublicKey(nil), private.Public().(ed25519.PublicKey)...), nil
}

func (s *FileSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, private, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	defer clear(private)
	return ed25519.Sign(private, message), nil
}

func (s *FileSigner) load(ctx context.Context) (fileSignerRecord, ed25519.PrivateKey, error) {
	if err := ctx.Err(); err != nil {
		return fileSignerRecord{}, nil, err
	}
	raw, found, err := storage.ReadStrictPrivateFileBounded(s.path, maxSignerFileBytes)
	if err != nil || !found {
		return fileSignerRecord{}, nil, ErrUnavailable
	}
	defer clear(raw)
	var record fileSignerRecord
	if err := storage.DecodeJSONStrict(raw, &record); err != nil {
		return fileSignerRecord{}, nil, ErrCorruptState
	}
	if record.Version != ContractVersion {
		return fileSignerRecord{}, nil, ErrUnsupportedVersion
	}
	private, err := base64.RawURLEncoding.Strict().DecodeString(record.PrivateKey)
	if err != nil || len(private) != ed25519.PrivateKeySize ||
		base64.RawURLEncoding.EncodeToString(private) != record.PrivateKey {
		clear(private)
		return fileSignerRecord{}, nil, ErrCorruptState
	}
	derived, err := identityprincipal.FromEd25519PublicKey(ed25519.PrivateKey(private).Public().(ed25519.PublicKey))
	if err != nil || derived.String() != record.Principal {
		clear(private)
		return fileSignerRecord{}, nil, ErrCorruptState
	}
	return record, ed25519.PrivateKey(private), nil
}
