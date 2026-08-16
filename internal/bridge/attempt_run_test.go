package bridge_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
)

func TestAcquireOwnsRetryAndDurableCompletion(t *testing.T) {
	fixture := newFixture(t)
	started := time.Now()
	config := fixture.config()
	config.Clock = func() time.Time { return fixture.now.Add(time.Since(started)) }
	owner, err := bridge.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil {
		t.Fatal(err)
	}
	manifest := sha256.Sum256([]byte("acquire manifest"))
	opener := &sequenceOpener{}
	channel, cleanup, err := owner.Acquire(context.Background(), transitionFrame(manifest), manifest,
		fixture.now.Add(time.Minute), opener.openContact)
	if err != nil {
		t.Fatal(err)
	}
	if opener.starts != 2 {
		t.Fatalf("contact starts = %d, want 2", opener.starts)
	}
	if _, err := channel.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	evidence, err := owner.Evidence()
	if err != nil {
		t.Fatal(err)
	}
	if evidence.ContactStarts != 2 || evidence.Terminal != "opened" || !evidence.CleanupComplete {
		t.Fatalf("attempt evidence = %+v", evidence)
	}
}

type sequenceOpener struct {
	starts int
}

func (opener *sequenceOpener) openContact(context.Context, [32]byte, []byte, time.Time) (
	net.Conn, func() error, bool, error,
) {
	opener.starts++
	if opener.starts == 1 {
		return nil, nil, true, errors.New("injected first-contact failure")
	}
	client, server := net.Pipe()
	go func() {
		buffer := make([]byte, 2)
		_, _ = server.Read(buffer)
		_ = server.Close()
	}()
	return client, client.Close, false, nil
}
