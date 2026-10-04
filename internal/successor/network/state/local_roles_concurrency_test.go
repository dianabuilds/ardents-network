package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	duty2 "github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func TestSourceExposureRetentionWaitsForConflictReader(t *testing.T) {
	root := filepath.Join(t.TempDir(), "roles")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	reader, err := duty2.Open(duty2.Config{Root: root, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	state := &networkState{config: config{root: t.TempDir(), localRoles: root, clock: time.Now,
		sourceInfo: source.Details{Identities: [2][32]byte{{1}, {2}}, Families: [2]string{"source-a", "source-b"}}}}
	done := make(chan error, 1)
	go func() { done <- state.retainSourceExposures(time.Now().Add(time.Hour)) }()
	select {
	case err := <-done:
		t.Fatalf("source write failed on concurrent reader: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := reader.Close(); err != nil {
		<-done
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source write did not join")
	}
	for _, identity := range state.config.sourceInfo.Identities {
		conflict, err := duty2.ReadConflict(root, time.Now, identity, [32]byte{})
		if err != nil || !conflict {
			t.Fatalf("lost Source exposure: %v, %v", conflict, err)
		}
	}
}
