package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	identityAccessFileName       = "identity-access.db"
	identityAccessMetadataBucket = "__storage_schema"
	identityAccessSchemaKey      = "version"
)

var (
	ErrDatabaseInUse       = errors.New("identity access database is already in use")
	ErrDatabaseClosing     = errors.New("identity access database is closing")
	ErrDatabaseClosed      = errors.New("identity access database is closed")
	ErrInvalidSchema       = errors.New("invalid identity access schema")
	ErrUnsupportedSchema   = errors.New("unsupported identity access schema version")
	ErrUnknownBucket       = errors.New("unknown identity access bucket")
	ErrTransactionPanicked = errors.New("identity access transaction panicked")
	ErrBackupExists        = errors.New("identity access backup already exists")
	ErrDatabaseOperation   = errors.New("identity access database operation failed")
)

// ReadTransaction is valid only for the duration of its Database callback.
// Returned keys and values are copies and remain owned by the caller.
type ReadTransaction interface {
	Get(bucket string, key []byte) ([]byte, bool, error)
	ForEach(bucket string, visit func(key, value []byte) error) error
}

// WriteTransaction is a bounded read/write view over schema-declared buckets.
type WriteTransaction interface {
	ReadTransaction
	Put(bucket string, key, value []byte) error
	Delete(bucket string, key []byte) error
}

// Database is the transaction seam consumed by identity repositories.
type Database interface {
	View(context.Context, func(ReadTransaction) error) error
	Update(context.Context, func(WriteTransaction) error) error
}

// Migration declares one consecutive schema version. Buckets are created before
// Apply runs, in the same transaction as the recorded version change.
type Migration struct {
	Version uint32
	Buckets []string
	Apply   func(WriteTransaction) error
}

// Schema is a complete, consecutive migration history starting at version one.
type Schema struct {
	Version    uint32
	Migrations []Migration
}

// BaseIdentityAccessSchema reserves the lifecycle schema. Identity Access adds
// its owner buckets as consecutive migrations as repositories are introduced.
func BaseIdentityAccessSchema() Schema {
	return Schema{Version: 1, Migrations: []Migration{{Version: 1}}}
}

// Handle owns the sole long-lived identity-access.db descriptor for a daemon.
// Close first stops transaction admission, then drains active callbacks.
type Handle struct {
	db      *bbolt.DB
	buckets map[string]struct{}
	publish func(string, string) error

	mu      sync.Mutex
	active  int
	closing bool
	closed  bool
	drained chan struct{}
	closeMu sync.Mutex
}

type boltReadTransaction struct {
	tx      *bbolt.Tx
	buckets map[string]struct{}
}

type boltWriteTransaction struct{ boltReadTransaction }

func IdentityAccessPathInDir(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, identityAccessFileName)
}

// OpenIdentityAccess opens and migrates the dedicated identity-access database.
// It never opens or changes the separate content and runtime database.
func OpenIdentityAccess(ctx context.Context, dir string, schema Schema) (*Handle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buckets, err := validateSchema(schema)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("%w: state directory is required", ErrInvalidSchema)
	}
	if err := EnsurePrivateDir(dir); err != nil {
		return nil, fmt.Errorf("prepare identity access database directory: %w", err)
	}
	path := IdentityAccessPathInDir(dir)
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Millisecond})
	if err != nil {
		if errors.Is(err, bbolt.ErrTimeout) {
			return nil, ErrDatabaseInUse
		}
		return nil, fmt.Errorf("open identity access database: %w", translateBoltError(err))
	}
	cleanup := func(openErr error) (*Handle, error) {
		_ = db.Close()
		return nil, openErr
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return cleanup(fmt.Errorf("protect identity access database: %w", err))
	}
	if err := ProtectPrivateFile(path); err != nil {
		return cleanup(fmt.Errorf("protect identity access database: %w", err))
	}
	if err := migrateIdentityAccess(ctx, db, schema, buckets); err != nil {
		return cleanup(err)
	}
	if err := verifyIdentityAccessBuckets(db, buckets); err != nil {
		return cleanup(err)
	}
	drained := make(chan struct{})
	close(drained)
	return &Handle{db: db, buckets: buckets, publish: publishPrivateFileNoReplace, drained: drained}, nil
}

func verifyIdentityAccessBuckets(db *bbolt.DB, buckets map[string]struct{}) error {
	err := db.View(func(tx *bbolt.Tx) error {
		for bucket := range buckets {
			if tx.Bucket([]byte(bucket)) == nil {
				return ErrUnsupportedSchema
			}
		}
		return nil
	})
	return translateBoltError(err)
}

func validateSchema(schema Schema) (map[string]struct{}, error) {
	if schema.Version == 0 || uint32(len(schema.Migrations)) != schema.Version {
		return nil, ErrInvalidSchema
	}
	buckets := make(map[string]struct{})
	for index, migration := range schema.Migrations {
		if migration.Version != uint32(index+1) {
			return nil, ErrInvalidSchema
		}
		for _, bucket := range migration.Buckets {
			if bucket == "" || bucket == identityAccessMetadataBucket || strings.TrimSpace(bucket) != bucket {
				return nil, ErrInvalidSchema
			}
			if _, exists := buckets[bucket]; exists {
				return nil, ErrInvalidSchema
			}
			buckets[bucket] = struct{}{}
		}
	}
	return buckets, nil
}

func migrateIdentityAccess(ctx context.Context, db *bbolt.DB, schema Schema, buckets map[string]struct{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var current uint32
	if err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(identityAccessMetadataBucket))
		if bucket == nil {
			return nil
		}
		raw := bucket.Get([]byte(identityAccessSchemaKey))
		if len(raw) != 4 {
			return ErrUnsupportedSchema
		}
		current = binary.BigEndian.Uint32(raw)
		return nil
	}); err != nil {
		return translateBoltError(err)
	}
	if current > schema.Version {
		return ErrUnsupportedSchema
	}
	if current == schema.Version {
		return nil
	}
	err := db.Update(func(tx *bbolt.Tx) (returnErr error) {
		defer func() {
			if recover() != nil {
				returnErr = ErrTransactionPanicked
			}
		}()
		metadata, err := tx.CreateBucketIfNotExists([]byte(identityAccessMetadataBucket))
		if err != nil {
			return translateBoltError(err)
		}
		wrapped := boltWriteTransaction{boltReadTransaction{tx: tx, buckets: buckets}}
		for _, migration := range schema.Migrations[current:] {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, bucket := range migration.Buckets {
				if _, err := tx.CreateBucket([]byte(bucket)); err != nil {
					return translateBoltError(err)
				}
			}
			if migration.Apply != nil {
				if err := migration.Apply(&wrapped); err != nil {
					return err
				}
			}
			if err := metadata.Put([]byte(identityAccessSchemaKey), encodeSchemaVersion(migration.Version)); err != nil {
				return translateBoltError(err)
			}
		}
		return ctx.Err()
	})
	return translateBoltError(err)
}

func encodeSchemaVersion(version uint32) []byte {
	raw := make([]byte, 4)
	binary.BigEndian.PutUint32(raw, version)
	return raw
}

func (h *Handle) View(ctx context.Context, callback func(ReadTransaction) error) error {
	if callback == nil {
		return fmt.Errorf("identity access View callback is required")
	}
	if err := h.admit(); err != nil {
		return err
	}
	defer h.finish()
	if err := ctx.Err(); err != nil {
		return err
	}
	err := h.db.View(func(tx *bbolt.Tx) (returnErr error) {
		defer func() {
			if recover() != nil {
				returnErr = ErrTransactionPanicked
			}
		}()
		return callback(&boltReadTransaction{tx: tx, buckets: h.buckets})
	})
	return translateBoltError(err)
}

func (h *Handle) Update(ctx context.Context, callback func(WriteTransaction) error) error {
	if callback == nil {
		return fmt.Errorf("identity access Update callback is required")
	}
	if err := h.admit(); err != nil {
		return err
	}
	defer h.finish()
	if err := ctx.Err(); err != nil {
		return err
	}
	err := h.db.Update(func(tx *bbolt.Tx) (returnErr error) {
		defer func() {
			if recover() != nil {
				returnErr = ErrTransactionPanicked
			}
		}()
		if err := callback(&boltWriteTransaction{boltReadTransaction{tx: tx, buckets: h.buckets}}); err != nil {
			return err
		}
		return ctx.Err()
	})
	return translateBoltError(err)
}

func (h *Handle) admit() error {
	if h == nil {
		return ErrDatabaseClosed
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return ErrDatabaseClosed
	}
	if h.closing {
		return ErrDatabaseClosing
	}
	if h.active == 0 {
		h.drained = make(chan struct{})
	}
	h.active++
	return nil
}

func (h *Handle) finish() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active--
	if h.active == 0 {
		close(h.drained)
	}
}

// Close stops admission, waits for admitted transactions, and closes the file.
// A cancelled Close can be retried; admission remains stopped after cancellation.
func (h *Handle) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.closeMu.Lock()
	defer h.closeMu.Unlock()
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closing = true
	drained := h.drained
	h.mu.Unlock()
	select {
	case <-drained:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := h.db.Close(); err != nil && !errors.Is(err, bbolt.ErrDatabaseNotOpen) {
		return fmt.Errorf("close identity access database")
	}
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	return nil
}

// Backup copies one committed read-transaction boundary to a new destination.
// Whole-state tooling must still stop the daemon to group this file with ardents.db.
func (h *Handle) Backup(ctx context.Context, destination string) (returnErr error) {
	if destination == "" {
		return fmt.Errorf("identity access backup destination is required")
	}
	if _, err := os.Stat(destination); err == nil {
		return ErrBackupExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := EnsurePrivateDir(filepath.Dir(destination)); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".identity-access-backup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := h.admit(); err != nil {
		return err
	}
	defer h.finish()
	if err := ctx.Err(); err != nil {
		return err
	}
	err = h.db.View(func(tx *bbolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return tx.CopyFile(temporaryPath, 0o600)
	})
	if err != nil {
		return translateBoltError(err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	completed, err := os.OpenFile(temporaryPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open completed identity access backup: %w", err)
	}
	if err := completed.Sync(); err != nil {
		_ = completed.Close()
		return fmt.Errorf("sync identity access backup: %w", err)
	}
	if err := completed.Close(); err != nil {
		return fmt.Errorf("close completed identity access backup: %w", err)
	}
	if err := ProtectPrivateFile(temporaryPath); err != nil {
		return err
	}
	if err := h.publish(temporaryPath, destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrBackupExists
		}
		return fmt.Errorf("install identity access backup: %w", err)
	}
	return nil
}

func (tx *boltReadTransaction) Get(bucket string, key []byte) ([]byte, bool, error) {
	current, err := tx.bucket(bucket)
	if err != nil {
		return nil, false, err
	}
	value := current.Get(key)
	if value == nil {
		return nil, false, nil
	}
	return append([]byte(nil), value...), true, nil
}

func (tx *boltReadTransaction) ForEach(bucket string, visit func(key, value []byte) error) error {
	if visit == nil {
		return fmt.Errorf("identity access visitor is required")
	}
	current, err := tx.bucket(bucket)
	if err != nil {
		return err
	}
	return translateBoltError(current.ForEach(func(key, value []byte) error {
		if value == nil {
			return ErrUnknownBucket
		}
		return visit(append([]byte(nil), key...), append([]byte(nil), value...))
	}))
}

func (tx *boltReadTransaction) bucket(name string) (*bbolt.Bucket, error) {
	if _, ok := tx.buckets[name]; !ok {
		return nil, ErrUnknownBucket
	}
	bucket := tx.tx.Bucket([]byte(name))
	if bucket == nil {
		return nil, ErrUnsupportedSchema
	}
	return bucket, nil
}

func (tx *boltWriteTransaction) Put(bucket string, key, value []byte) error {
	current, err := tx.bucket(bucket)
	if err != nil {
		return err
	}
	return translateBoltError(current.Put(key, value))
}

func (tx *boltWriteTransaction) Delete(bucket string, key []byte) error {
	current, err := tx.bucket(bucket)
	if err != nil {
		return err
	}
	return translateBoltError(current.Delete(key))
}

func translateBoltError(err error) error {
	if err == nil {
		return nil
	}
	for _, preserved := range []error{context.Canceled, context.DeadlineExceeded, ErrUnsupportedSchema, ErrUnknownBucket, ErrTransactionPanicked} {
		if errors.Is(err, preserved) {
			return preserved
		}
	}
	boltErrors := []error{
		bbolt.ErrDatabaseNotOpen, bbolt.ErrInvalid,
		bbolt.ErrVersionMismatch, bbolt.ErrChecksum, bbolt.ErrTimeout,
		bbolt.ErrTxNotWritable, bbolt.ErrTxClosed, bbolt.ErrDatabaseReadOnly,
		bbolt.ErrBucketNotFound, bbolt.ErrBucketExists, bbolt.ErrBucketNameRequired,
		bbolt.ErrKeyRequired, bbolt.ErrKeyTooLarge, bbolt.ErrValueTooLarge,
		bbolt.ErrIncompatibleValue,
	}
	for _, boltErr := range boltErrors {
		if errors.Is(err, boltErr) {
			return ErrDatabaseOperation
		}
	}
	return err
}
