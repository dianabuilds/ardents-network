package authority

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"ardents/internal/storage"
)

type FileCheckpointRepository struct {
	root string
	mu   sync.Mutex
}

const (
	maxCheckpointFileBytes int64 = 8192
	wormMarkerFile               = ".ardents-worm-repository-v1.json"
)

var expectedWORMMarker = wormRepositoryMarker{
	Version: 1, Retention: "worm", Administration: "independent",
}

type wormRepositoryMarker struct {
	Version        uint32 `json:"version"`
	Retention      string `json:"retention"`
	Administration string `json:"administration"`
}

func NewFileCheckpointRepository(root string) (*FileCheckpointRepository, error) {
	return newFileCheckpointRepository(root, false)
}

// NewWORMFileCheckpointRepository is the production admission boundary. The
// repository root and marker must be provisioned by the independent storage
// administrator; the daemon never creates either and has no local fallback.
func NewWORMFileCheckpointRepository(root string) (*FileCheckpointRepository, error) {
	return newFileCheckpointRepository(root, true)
}

func newFileCheckpointRepository(root string, requireWORM bool) (*FileCheckpointRepository, error) {
	if filepath.Clean(root) == "." || root == "" {
		return nil, ErrInvalidArgument
	}
	if err := validatePreprovisionedPrivateDir(root); err != nil {
		return nil, fmt.Errorf("%w: checkpoint repository", ErrUnavailable)
	}
	if requireWORM {
		raw, found, err := storage.ReadStrictPrivateFileBounded(
			filepath.Join(root, wormMarkerFile), 512,
		)
		if err != nil || !found {
			return nil, fmt.Errorf("%w: checkpoint WORM assertion", ErrUnavailable)
		}
		var marker wormRepositoryMarker
		if err := storage.DecodeJSONStrict(raw, &marker); err != nil || marker != expectedWORMMarker {
			return nil, fmt.Errorf("%w: invalid checkpoint WORM assertion", ErrUnavailable)
		}
	}
	return &FileCheckpointRepository{root: filepath.Clean(root)}, nil
}

func (r *FileCheckpointRepository) ReadHead(ctx context.Context, realmID string) (SignedCheckpoint, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readHead(ctx, realmID)
}

func (r *FileCheckpointRepository) CreateIfAbsent(ctx context.Context, next SignedCheckpoint) (SignedCheckpoint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SignedCheckpoint{}, err
	}
	if err := ValidateCheckpoint(next); err != nil || next.AuthoritySequence != 1 || next.PreviousDigest != "" {
		return SignedCheckpoint{}, ErrInvalidArgument
	}
	if _, found, err := r.readHead(ctx, next.RealmID); err != nil {
		return SignedCheckpoint{}, err
	} else if found {
		return SignedCheckpoint{}, ErrConflict
	}
	if err := r.writeImmutable(next); err != nil {
		if errors.Is(err, os.ErrExist) {
			return SignedCheckpoint{}, ErrConflict
		}
		return SignedCheckpoint{}, err
	}
	return next, nil
}

func (r *FileCheckpointRepository) CompareAndAppend(ctx context.Context, realmID string, expected uint64, next SignedCheckpoint) (SignedCheckpoint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SignedCheckpoint{}, err
	}
	head, found, err := r.readHead(ctx, realmID)
	if err != nil {
		return SignedCheckpoint{}, err
	}
	if !found || expected != head.AuthoritySequence || next.RealmID != realmID ||
		next.AuthoritySequence != expected+1 || next.PreviousDigest != head.Digest ||
		!validCheckpointAuthoritySuccessor(head, next) {
		return SignedCheckpoint{}, ErrConflict
	}
	if expected >= MaxCheckpointRecords {
		return SignedCheckpoint{}, ErrResourceExhausted
	}
	if err := ValidateCheckpoint(next); err != nil {
		return SignedCheckpoint{}, ErrInvalidArgument
	}
	if err := r.writeImmutable(next); err != nil {
		if errors.Is(err, os.ErrExist) {
			return SignedCheckpoint{}, ErrConflict
		}
		return SignedCheckpoint{}, err
	}
	return next, nil
}

func validCheckpointAuthoritySuccessor(head, next SignedCheckpoint) bool {
	if next.AuthorityPrincipal == head.AuthorityPrincipal &&
		equalBytes(next.AuthorityPublicKey, head.AuthorityPublicKey) {
		return next.AuthorityEpoch == head.AuthorityEpoch &&
			next.AuthorityTransition == nil
	}
	if next.AuthorityTransition == nil {
		return false
	}
	transition := *next.AuthorityTransition
	return ValidateAuthorityTransition(transition) == nil &&
		transition.RealmID == head.RealmID &&
		transition.FromAuthorityPrincipal == head.AuthorityPrincipal &&
		equalBytes(transition.FromAuthorityPublicKey, head.AuthorityPublicKey) &&
		transition.FromAuthorityEpoch == head.AuthorityEpoch &&
		transition.AuthoritySequence == head.AuthoritySequence &&
		transition.CheckpointDigest == head.Digest &&
		transition.ToAuthorityPrincipal == next.AuthorityPrincipal &&
		equalBytes(transition.ToAuthorityPublicKey, next.AuthorityPublicKey) &&
		transition.ToAuthorityEpoch == next.AuthorityEpoch
}

func (r *FileCheckpointRepository) readHead(ctx context.Context, realmID string) (SignedCheckpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return SignedCheckpoint{}, false, err
	}
	if !ValidRealmID(realmID) {
		return SignedCheckpoint{}, false, ErrInvalidArgument
	}
	dir := filepath.Join(r.root, realmID)
	if _, statErr := os.Lstat(dir); statErr == nil {
		if err := validatePreprovisionedPrivateDir(dir); err != nil {
			return SignedCheckpoint{}, false, ErrCorruptState
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return SignedCheckpoint{}, false, ErrUnavailable
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return SignedCheckpoint{}, false, nil
	}
	if err != nil {
		return SignedCheckpoint{}, false, ErrUnavailable
	}
	if len(entries) > MaxCheckpointRecords {
		return SignedCheckpoint{}, false, ErrCorruptState
	}
	var latest SignedCheckpoint
	var found bool
	var previous uint64
	var previousDigest string
	for _, entry := range entries {
		if entry.IsDir() {
			return SignedCheckpoint{}, false, ErrCorruptState
		}
		raw, exists, err := storage.ReadPrivateFileBounded(
			filepath.Join(dir, entry.Name()), maxCheckpointFileBytes,
		)
		if err != nil || !exists {
			return SignedCheckpoint{}, false, ErrUnavailable
		}
		var checkpoint SignedCheckpoint
		if err := storage.DecodeJSONStrict(raw, &checkpoint); err != nil ||
			ValidateCheckpoint(checkpoint) != nil || checkpoint.RealmID != realmID {
			return SignedCheckpoint{}, false, ErrCorruptState
		}
		if checkpoint.AuthoritySequence != previous+1 ||
			entry.Name() != fmt.Sprintf("%020d.json", checkpoint.AuthoritySequence) ||
			(previous > 0 && (checkpoint.PreviousDigest != previousDigest ||
				!validCheckpointAuthoritySuccessor(latest, checkpoint))) {
			return SignedCheckpoint{}, false, ErrCorruptState
		}
		latest, found = checkpoint, true
		previous, previousDigest = checkpoint.AuthoritySequence, checkpoint.Digest
	}
	return latest, found, nil
}

func (r *FileCheckpointRepository) writeImmutable(checkpoint SignedCheckpoint) error {
	dir := filepath.Join(r.root, checkpoint.RealmID)
	if err := os.Mkdir(dir, 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return ErrUnavailable
		}
	} else {
		if err := storage.EnsurePrivateDir(dir); err != nil {
			return ErrUnavailable
		}
	}
	if err := validatePreprovisionedPrivateDir(dir); err != nil {
		return ErrUnavailable
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return ErrInvalidArgument
	}
	name := fmt.Sprintf("%020d.json", checkpoint.AuthoritySequence)
	path := filepath.Join(dir, name)
	if err := storage.AtomicCreatePrivateFile(path, raw); err != nil {
		return err
	}
	return nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for index := range left {
		diff |= left[index] ^ right[index]
	}
	return diff == 0
}
