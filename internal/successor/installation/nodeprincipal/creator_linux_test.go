package nodeprincipal

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestCreatorRefusesEffectsWithoutOriginalPreparation(t *testing.T) {
	creator, err := PrepareCreator(nil)
	if !errors.Is(err, ErrInput) || creator == nil {
		t.Fatalf("nil original caller: %v", err)
	}
	if err := creator.Execute(); !errors.Is(err, ErrInput) {
		t.Fatalf("unprepared helper admitted: %v", err)
	}
	if _, err := creator.Preimages(); !errors.Is(err, ErrInput) {
		t.Fatalf("unprepared preimages admitted: %v", err)
	}
	if _, _, _, err := creator.SyncResult("/etc/passwd"); !errors.Is(err, ErrInput) {
		t.Fatalf("unprepared sync admitted: %v", err)
	}
	if err := creator.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Observe(ctx, 999, 999); !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation lost: %v", err)
	}
}

type accountTestInfo struct{ stat syscall.Stat_t }

func (i accountTestInfo) Name() string       { return "account" }
func (i accountTestInfo) Size() int64        { return 1 }
func (i accountTestInfo) Mode() os.FileMode  { return 0600 }
func (i accountTestInfo) ModTime() time.Time { return time.Time{} }
func (i accountTestInfo) IsDir() bool        { return false }
func (i accountTestInfo) Sys() any           { return &i.stat }

func TestCreatorPreimagesKeepFixedOrderAndAbsentAuxiliary(t *testing.T) {
	a := &Creator{prepared: true, before: map[string]*accountFile{}, auxBefore: map[string]*accountFile{}}
	for _, name := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		a.before[name] = &accountFile{info: accountTestInfo{stat: syscall.Stat_t{Uid: 0, Gid: 0}}, raw: []byte("x")}
	}
	records, err := a.Preimages()
	if err != nil || len(records) != 4+len(AuxiliaryPaths()) {
		t.Fatalf("preimage inventory: %d, %v", len(records), err)
	}
	if records[0].Path != "/etc/passwd" || records[3].Path != "/etc/gshadow" || records[0].SHA256 != "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881" {
		t.Fatalf("preimage order or digest changed: %+v", records[0])
	}
	if records[4].Path != "/etc/.pwd.lock" || records[4].Info != nil || records[4].SHA256 != "" {
		t.Fatalf("absent auxiliary not retained: %+v", records[4])
	}
	a.completed = true
	if _, _, _, err := a.SyncResult("/etc/foreign"); !errors.Is(err, ErrInput) {
		t.Fatalf("foreign path admitted: %v", err)
	}
}
