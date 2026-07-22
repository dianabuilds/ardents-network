package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func testIdentityAccessSchema() Schema {
	return Schema{Version: 2, Migrations: []Migration{
		{Version: 1, Buckets: []string{"records"}},
		{Version: 2, Buckets: []string{"indexes"}, Apply: func(tx WriteTransaction) error {
			return tx.Put("records", []byte("migrated"), []byte("yes"))
		}},
	}}
}

func openTestIdentityAccess(t *testing.T) *Handle {
	t.Helper()
	database, err := OpenIdentityAccess(context.Background(), t.TempDir(), testIdentityAccessSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	return database
}

func TestIdentityAccessPathIsSeparateFromLegacyDatabase(t *testing.T) {
	dir := t.TempDir()
	require.Equal(t, filepath.Join(dir, "identity-access.db"), IdentityAccessPathInDir(dir))
	require.NotEqual(t, PathInDir(dir), IdentityAccessPathInDir(dir))
}

func TestIdentityAccessSchemaMigratesAndRejectsUnknownOrIncompleteVersions(t *testing.T) {
	dir := t.TempDir()
	database, err := OpenIdentityAccess(context.Background(), dir, testIdentityAccessSchema())
	require.NoError(t, err)
	require.NoError(t, database.View(context.Background(), func(tx ReadTransaction) error {
		value, found, err := tx.Get("records", []byte("migrated"))
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, []byte("yes"), value)
		return nil
	}))
	require.NoError(t, database.Close(context.Background()))

	reopened, err := OpenIdentityAccess(context.Background(), dir, testIdentityAccessSchema())
	require.NoError(t, err)
	require.NoError(t, reopened.Close(context.Background()))

	for name, version := range map[string]uint32{"unknown": 3, "maximum": ^uint32(0)} {
		t.Run(name, func(t *testing.T) {
			path := IdentityAccessPathInDir(t.TempDir())
			raw, err := bbolt.Open(path, 0o600, nil)
			require.NoError(t, err)
			require.NoError(t, raw.Update(func(tx *bbolt.Tx) error {
				bucket, err := tx.CreateBucket([]byte(identityAccessMetadataBucket))
				if err != nil {
					return err
				}
				return bucket.Put([]byte(identityAccessSchemaKey), encodeSchemaVersion(version))
			}))
			require.NoError(t, raw.Close())
			_, err = OpenIdentityAccess(context.Background(), filepath.Dir(path), testIdentityAccessSchema())
			require.ErrorIs(t, err, ErrUnsupportedSchema)
		})
	}

	_, err = OpenIdentityAccess(context.Background(), t.TempDir(), Schema{Version: 2, Migrations: []Migration{{Version: 2}}})
	require.ErrorIs(t, err, ErrInvalidSchema)

	damagedDir := t.TempDir()
	damagedPath := IdentityAccessPathInDir(damagedDir)
	raw, err := bbolt.Open(damagedPath, 0o600, nil)
	require.NoError(t, err)
	require.NoError(t, raw.Update(func(tx *bbolt.Tx) error {
		metadata, err := tx.CreateBucket([]byte(identityAccessMetadataBucket))
		if err != nil {
			return err
		}
		return metadata.Put([]byte(identityAccessSchemaKey), encodeSchemaVersion(2))
	}))
	require.NoError(t, raw.Close())
	_, err = OpenIdentityAccess(context.Background(), damagedDir, testIdentityAccessSchema())
	require.ErrorIs(t, err, ErrUnsupportedSchema)
}

func TestIdentityAccessHandleIsExclusiveAndReopenable(t *testing.T) {
	dir := t.TempDir()
	first, err := OpenIdentityAccess(context.Background(), dir, testIdentityAccessSchema())
	require.NoError(t, err)
	_, err = OpenIdentityAccess(context.Background(), dir, testIdentityAccessSchema())
	require.ErrorIs(t, err, ErrDatabaseInUse)
	raw, err := bbolt.Open(IdentityAccessPathInDir(dir), 0o600, &bbolt.Options{Timeout: time.Millisecond})
	require.ErrorIs(t, err, bbolt.ErrTimeout)
	require.Nil(t, raw)
	require.NoError(t, first.Close(context.Background()))
	second, err := OpenIdentityAccess(context.Background(), dir, testIdentityAccessSchema())
	require.NoError(t, err)
	require.NoError(t, second.Close(context.Background()))
}

func TestIdentityAccessTransactionsAreIsolatedAndWriterIsSerialized(t *testing.T) {
	database := openTestIdentityAccess(t)
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("counter"), []byte("0"))
	}))

	const writers = 20
	started := make(chan struct{}, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			started <- struct{}{}
			require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
				value, found, err := tx.Get("records", []byte("counter"))
				if err != nil || !found {
					return fmt.Errorf("read counter: %w", err)
				}
				var current int
				_, err = fmt.Sscanf(string(value), "%d", &current)
				if err != nil {
					return err
				}
				return tx.Put("records", []byte("counter"), []byte(fmt.Sprint(current+1)))
			}))
		}()
	}
	for index := 0; index < writers; index++ {
		<-started
	}
	wait.Wait()
	require.NoError(t, database.View(context.Background(), func(tx ReadTransaction) error {
		value, found, err := tx.Get("records", []byte("counter"))
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "20", string(value))
		return nil
	}))
}

func TestIdentityAccessReaderKeepsSnapshotAcrossConcurrentCommit(t *testing.T) {
	database := openTestIdentityAccess(t)
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("state"), []byte("before"))
	}))
	readerEntered := make(chan struct{})
	writerEntered := make(chan struct{})
	releaseWriter := make(chan struct{})
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- database.View(context.Background(), func(tx ReadTransaction) error {
			first, _, err := tx.Get("records", []byte("state"))
			if err != nil {
				return err
			}
			close(readerEntered)
			<-writerEntered
			second, _, err := tx.Get("records", []byte("state"))
			if err != nil {
				return err
			}
			if !bytes.Equal(first, second) || !bytes.Equal(second, []byte("before")) {
				return fmt.Errorf("reader snapshot changed from %q to %q", first, second)
			}
			close(releaseWriter)
			return nil
		})
	}()
	<-readerEntered
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- database.Update(context.Background(), func(tx WriteTransaction) error {
			if err := tx.Put("records", []byte("state"), []byte("after")); err != nil {
				return err
			}
			close(writerEntered)
			<-releaseWriter
			return nil
		})
	}()
	require.NoError(t, <-readerDone)
	require.NoError(t, <-writerDone)
}

func TestIdentityAccessUpdateRollsBackOnErrorPanicAndCancellation(t *testing.T) {
	database := openTestIdentityAccess(t)
	sentinel := errors.New("callback failed")
	err := database.Update(context.Background(), func(tx WriteTransaction) error {
		require.NoError(t, tx.Put("records", []byte("error"), []byte("must rollback")))
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	err = database.Update(context.Background(), func(tx WriteTransaction) error {
		require.NoError(t, tx.Put("records", []byte("panic"), []byte("must rollback")))
		panic("secret value")
	})
	require.ErrorIs(t, err, ErrTransactionPanicked)
	require.NotContains(t, err.Error(), "secret value")

	ctx, cancel := context.WithCancel(context.Background())
	err = database.Update(ctx, func(tx WriteTransaction) error {
		require.NoError(t, tx.Put("records", []byte("cancel"), []byte("must rollback")))
		cancel()
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)

	require.NoError(t, database.View(context.Background(), func(tx ReadTransaction) error {
		for _, key := range []string{"error", "panic", "cancel"} {
			_, found, getErr := tx.Get("records", []byte(key))
			require.NoError(t, getErr)
			require.False(t, found)
		}
		return nil
	}))
}

func TestIdentityAccessCancelledContextDoesNotInvokeCallback(t *testing.T) {
	database := openTestIdentityAccess(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := database.Update(ctx, func(WriteTransaction) error { called = true; return nil })
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called)
}

func TestIdentityAccessTransactionsRejectUnknownBucketsAndCopyValues(t *testing.T) {
	database := openTestIdentityAccess(t)
	err := database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("unknown", []byte("key"), []byte("value"))
	})
	require.ErrorIs(t, err, ErrUnknownBucket)

	source := []byte("value")
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("key"), source)
	}))
	source[0] = 'X'
	require.NoError(t, database.View(context.Background(), func(tx ReadTransaction) error {
		value, found, getErr := tx.Get("records", []byte("key"))
		require.NoError(t, getErr)
		require.True(t, found)
		value[0] = 'X'
		again, _, getErr := tx.Get("records", []byte("key"))
		require.NoError(t, getErr)
		require.Equal(t, []byte("value"), again)
		return nil
	}))
}

func TestIdentityAccessBackupIsOneTransactionBoundaryAndReopens(t *testing.T) {
	database := openTestIdentityAccess(t)
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("before"), []byte("present"))
	}))

	destination := filepath.Join(t.TempDir(), "identity-access.backup.db")
	require.NoError(t, database.Backup(context.Background(), destination))
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("after"), []byte("live only"))
	}))

	backup, err := bbolt.Open(destination, 0o600, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	require.NoError(t, backup.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("records"))
		require.NotNil(t, bucket)
		require.Equal(t, []byte("present"), bucket.Get([]byte("before")))
		require.Nil(t, bucket.Get([]byte("after")))
		return nil
	}))
	require.NoError(t, backup.Close())
	require.ErrorIs(t, database.Backup(context.Background(), destination), ErrBackupExists)
}

func TestIdentityAccessBackupPublishFailureLeavesNoDestinationAndCanRetry(t *testing.T) {
	database := openTestIdentityAccess(t)
	require.NoError(t, database.Update(context.Background(), func(tx WriteTransaction) error {
		return tx.Put("records", []byte("durable"), []byte("value"))
	}))
	destination := filepath.Join(t.TempDir(), "retry.db")
	originalPublish := database.publish
	database.publish = func(_, _ string) error { return errors.New("injected publish failure") }
	err := database.Backup(context.Background(), destination)
	require.ErrorContains(t, err, "install identity access backup")
	require.NoFileExists(t, destination)

	database.publish = originalPublish
	require.NoError(t, database.Backup(context.Background(), destination))
	reopened, err := bbolt.Open(destination, 0o600, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	require.NoError(t, reopened.View(func(tx *bbolt.Tx) error {
		require.Equal(t, []byte("value"), tx.Bucket([]byte("records")).Get([]byte("durable")))
		return nil
	}))
	require.NoError(t, reopened.Close())
}

func TestIdentityAccessCloseStopsAdmissionAndDrainsActiveTransaction(t *testing.T) {
	database := openTestIdentityAccess(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	transactionDone := make(chan error, 1)
	go func() {
		transactionDone <- database.View(context.Background(), func(ReadTransaction) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	closeDone := make(chan error, 1)
	go func() { closeDone <- database.Close(context.Background()) }()
	require.Eventually(t, func() bool {
		err := database.View(context.Background(), func(ReadTransaction) error { return nil })
		return errors.Is(err, ErrDatabaseClosing)
	}, time.Second, time.Millisecond)
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before transaction drained: %v", err)
	default:
	}
	close(release)
	require.NoError(t, <-transactionDone)
	require.NoError(t, <-closeDone)
	require.ErrorIs(t, database.Update(context.Background(), func(WriteTransaction) error { return nil }), ErrDatabaseClosed)
}

func TestIdentityAccessCloseHonorsContextWhileDraining(t *testing.T) {
	database := openTestIdentityAccess(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = database.View(context.Background(), func(ReadTransaction) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, database.Close(ctx), context.Canceled)
	close(release)
	require.NoError(t, database.Close(context.Background()))
}

func TestEncodeSchemaVersionIsStable(t *testing.T) {
	require.True(t, bytes.Equal([]byte{0, 0, 0, 2}, encodeSchemaVersion(2)))
}
