package endpoint

import (
	"context"
	"io"
	"net"
	"testing"
)

func TestLocalServiceAdministrationDispatchesOnlyClosedOperations(t *testing.T) {
	t.Parallel()
	path := shortApplicationPath(t)
	published, withdrawn := make(chan struct{}, 1), make(chan struct{}, 1)
	server, err := openLocalServiceAdministration(localServiceAdministrationConfig{
		Path: path,
		Publish: func(context.Context) error {
			published <- struct{}{}
			return nil
		},
		Withdraw: func(context.Context) error {
			withdrawn <- struct{}{}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	for operation, fixture := range map[string]struct {
		signal <-chan struct{}
		want   string
	}{"publish": {published, "published\n"}, "withdraw": {withdrawn, "withdrawn\n"}} {
		response := requestLocalServiceAdministration(t, path, operation+"\n")
		if response != fixture.want {
			t.Fatalf("%s response = %q", operation, response)
		}
		select {
		case <-fixture.signal:
		default:
			t.Fatalf("%s operation was not dispatched", operation)
		}
	}
	if response := requestLocalServiceAdministration(t, path, "publish\nsurplus"); response != "unavailable\n" {
		t.Fatalf("surplus operation response = %q", response)
	}
	select {
	case <-published:
		t.Fatal("surplus request dispatched publication")
	default:
	}
}

func requestLocalServiceAdministration(t *testing.T, path, request string) string {
	t.Helper()
	raw, err := (&net.Dialer{}).DialContext(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	connection := raw.(*net.UnixConn)
	defer connection.Close()
	if _, err := io.WriteString(connection, request); err != nil {
		t.Fatal(err)
	}
	if err := connection.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, 64))
	if err != nil {
		t.Fatal(err)
	}
	return string(response)
}
