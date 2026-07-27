package authority

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ardents/internal/storage"

	"go.etcd.io/bbolt"
)

const (
	AuthorityStoreKeyBytes              = 32
	authorityStoreFileVersion    uint32 = 1
	authorityStoreMetadataBucket        = "__authority_schema"
	authorityStoreLedgerBucket          = "realm_authority_ledger"
	authorityStoreVersionKey            = "version"
	authorityStoreLedgerKey             = "primary"
	authorityStoreCipherDomain          = "ardents:realm-authority-store:v1\x00"
	maxAuthorityLedgerBytes             = 8 << 20
)

type FileStore struct {
	db     *bbolt.DB
	aead   cipher.AEAD
	mu     sync.Mutex
	closed bool
}

func OpenFileStore(ctx context.Context, path string, key []byte) (*FileStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || len(key) != AuthorityStoreKeyBytes {
		return nil, ErrInvalidArgument
	}
	if err := validatePreprovisionedPrivateDir(filepath.Dir(path)); err != nil {
		return nil, ErrUnavailable
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if err := storage.AtomicCreatePrivateFile(path, nil); err != nil {
			return nil, ErrUnavailable
		}
	} else if err != nil || storage.ValidateStrictPrivateFile(path) != nil {
		return nil, ErrUnavailable
	}
	block, err := aes.NewCipher(append([]byte(nil), key...))
	if err != nil {
		return nil, ErrInvalidArgument
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidArgument
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Millisecond})
	if err != nil {
		return nil, ErrUnavailable
	}
	cleanup := func(cause error) (*FileStore, error) {
		_ = db.Close()
		return nil, cause
	}
	if err := storage.ValidateStrictPrivateFile(path); err != nil {
		return cleanup(ErrUnavailable)
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		metadata, err := tx.CreateBucketIfNotExists([]byte(authorityStoreMetadataBucket))
		if err != nil {
			return err
		}
		raw := metadata.Get([]byte(authorityStoreVersionKey))
		switch len(raw) {
		case 0:
			var encoded [4]byte
			binary.BigEndian.PutUint32(encoded[:], authorityStoreFileVersion)
			if err := metadata.Put([]byte(authorityStoreVersionKey), encoded[:]); err != nil {
				return err
			}
		case 4:
			if binary.BigEndian.Uint32(raw) != authorityStoreFileVersion {
				return ErrUnsupportedVersion
			}
		default:
			return ErrCorruptState
		}
		_, err = tx.CreateBucketIfNotExists([]byte(authorityStoreLedgerBucket))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrUnsupportedVersion) || errors.Is(err, ErrCorruptState) {
			return cleanup(err)
		}
		return cleanup(ErrUnavailable)
	}
	return &FileStore{db: db, aead: aead}, nil
}

func validatePreprovisionedPrivateDir(path string) error {
	if err := storage.ValidatePrivateDir(path); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(absolute)
	current := volume + string(os.PathSeparator)
	remainder := strings.TrimPrefix(absolute, current)
	for _, component := range strings.Split(remainder, string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("private state parent must not traverse symlinks")
		}
	}
	return nil
}

func (s *FileStore) Load(ctx context.Context) (Ledger, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.db == nil {
		return Ledger{}, false, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Ledger{}, false, err
	}
	var sealed []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(authorityStoreLedgerBucket))
		if bucket == nil {
			return ErrCorruptState
		}
		raw := bucket.Get([]byte(authorityStoreLedgerKey))
		if raw != nil {
			if len(raw) > maxAuthorityLedgerBytes+64 {
				return ErrCorruptState
			}
			sealed = append([]byte(nil), raw...)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCorruptState) {
			return Ledger{}, false, err
		}
		return Ledger{}, false, ErrUnavailable
	}
	if len(sealed) == 0 {
		return Ledger{}, false, nil
	}
	raw, err := s.open(sealed)
	if err != nil {
		return Ledger{}, false, err
	}
	defer clear(raw)
	state, err := decodeLedger(raw)
	if err != nil {
		return Ledger{}, false, err
	}
	return state, true, nil
}

func (s *FileStore) Create(ctx context.Context, state Ledger) error {
	return s.write(ctx, 0, state, true)
}

func (s *FileStore) Save(ctx context.Context, expectedRevision uint64, state Ledger) error {
	return s.write(ctx, expectedRevision, state, false)
}

func (s *FileStore) write(ctx context.Context, expectedRevision uint64, state Ledger, create bool) error {
	if err := validateLedger(state); err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return ErrCorruptState
	}
	defer clear(raw)
	if len(raw) > maxAuthorityLedgerBytes {
		return ErrResourceExhausted
	}
	sealed, err := s.seal(raw)
	if err != nil {
		return err
	}
	defer clear(sealed)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.db == nil {
		return ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(authorityStoreLedgerBucket))
		if bucket == nil {
			return ErrCorruptState
		}
		current := bucket.Get([]byte(authorityStoreLedgerKey))
		if create {
			if current != nil {
				return ErrConflict
			}
		} else {
			if current == nil {
				return ErrConflict
			}
			plain, err := s.open(current)
			if err != nil {
				return err
			}
			defer clear(plain)
			prior, err := decodeLedger(plain)
			if err != nil {
				return err
			}
			if prior.Revision != expectedRevision || state.Revision != expectedRevision+1 ||
				prior.RealmID != state.RealmID {
				return ErrConflict
			}
		}
		return bucket.Put([]byte(authorityStoreLedgerKey), sealed)
	})
	if err == nil {
		return nil
	}
	for _, preserved := range []error{ErrConflict, ErrCorruptState, ErrUnsupportedVersion} {
		if errors.Is(err, preserved) {
			return preserved
		}
	}
	return ErrUnavailable
}

func (s *FileStore) seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrUnavailable
	}
	out := make([]byte, 0, len(nonce)+len(plain)+s.aead.Overhead())
	out = append(out, nonce...)
	out = s.aead.Seal(out, nonce, plain, []byte(authorityStoreCipherDomain))
	return out, nil
}

func (s *FileStore) open(sealed []byte) ([]byte, error) {
	nonceSize := s.aead.NonceSize()
	if len(sealed) < nonceSize+s.aead.Overhead() {
		return nil, ErrCorruptState
	}
	plain, err := s.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], []byte(authorityStoreCipherDomain))
	if err != nil {
		return nil, ErrCorruptState
	}
	return plain, nil
}

func decodeLedger(raw []byte) (Ledger, error) {
	if len(raw) > maxAuthorityLedgerBytes {
		return Ledger{}, ErrCorruptState
	}
	var state Ledger
	if err := storage.DecodeJSONStrict(raw, &state); err != nil {
		return Ledger{}, ErrCorruptState
	}
	if state.Version != ContractVersion || state.SchemaVersion != SchemaVersion {
		return Ledger{}, ErrUnsupportedVersion
	}
	if err := validateLedger(state); err != nil {
		return Ledger{}, err
	}
	return state, nil
}

func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil && !errors.Is(err, bbolt.ErrDatabaseNotOpen) {
		return fmt.Errorf("close authority store: %w", ErrUnavailable)
	}
	return nil
}
