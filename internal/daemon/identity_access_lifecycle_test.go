package daemon

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	identityaccess "ardents/internal/identity/access"
	"ardents/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestDaemonOwnersReceiveSingleIdentityAccessHandle(t *testing.T) {
	dir := t.TempDir()
	database, err := storage.OpenIdentityAccess(context.Background(), dir, identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })

	owners, err := newDaemonOwners(Config{Name: "alpha", Data: DataConfig{Dir: dir}}, database)
	require.NoError(t, err)
	require.Same(t, database, owners.IdentityAccess)
	require.NotNil(t, owners.PrincipalAccess)
}

func TestDaemonDrainsEveryListenerBeforeIdentityAccessShutdown(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), storage.BaseIdentityAccessSchema())
	require.NoError(t, err)
	listenerOne, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	listenerTwo, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	requestEntered := make(chan struct{})
	releaseRequest := make(chan struct{})
	serverOne := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		err := database.View(context.Background(), func(storage.ReadTransaction) error {
			close(requestEntered)
			<-releaseRequest
			return nil
		})
		require.NoError(t, err)
		writer.WriteHeader(http.StatusNoContent)
	})}
	serverTwo := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	ctx, cancel := context.WithCancel(context.Background())
	drained := make(chan error, 1)
	go func() {
		drained <- serveAndDrain(ctx, cancel, []*http.Server{serverOne, serverTwo},
			serveTarget{serve: func() error { return serverOne.Serve(listenerOne) }},
			serveTarget{serve: func() error { return serverTwo.Serve(listenerTwo) }})
	}()
	requestDone := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + listenerOne.Addr().String())
		if requestErr == nil {
			requestErr = response.Body.Close()
		}
		requestDone <- requestErr
	}()
	<-requestEntered
	cancel()
	select {
	case err := <-drained:
		t.Fatalf("listeners drained before active handler returned: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	require.NoError(t, database.View(context.Background(), func(storage.ReadTransaction) error { return nil }))
	close(releaseRequest)
	require.NoError(t, <-requestDone)
	require.NoError(t, <-drained)
	require.NoError(t, closeIdentityAccess(database))
}

func TestDaemonIdentityAccessShutdownDrainsBeforeReturning(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), storage.BaseIdentityAccessSchema())
	require.NoError(t, err)
	entered := make(chan struct{})
	release := make(chan struct{})
	transactionDone := make(chan error, 1)
	go func() {
		transactionDone <- database.View(context.Background(), func(storage.ReadTransaction) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- closeIdentityAccess(database) }()
	select {
	case err := <-shutdownDone:
		t.Fatalf("daemon database shutdown returned before drain: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-transactionDone)
	require.NoError(t, <-shutdownDone)
}
