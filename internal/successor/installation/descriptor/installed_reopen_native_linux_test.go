//go:build linux && descriptor_installed_native

package descriptor_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// The parent launches this independently under the original numeric Node UID
// after actual native exit. Existing bytes are required before OpenStore, and
// unchanged bytes/inodes after Close exclude initialization/repair as success.
// This supplies no Installation Stop, restart or replacement authority.
func TestDescriptorInstalledNativeStoreReopen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Node principal required")
	}
	const root = descriptorNativeParent + "/runtime/store"
	info, err := os.Stat(root)
	if err != nil || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		t.Fatal("original Store principal", err)
	}
	var public struct{ Target [32]byte }
	if err := json.Unmarshal(descriptorNativeRead(t, "exchange-prepared.json"), &public); err != nil {
		t.Fatal(err)
	}
	raw := descriptorNativeRead(t, "descriptor-proof-2.bin")
	if len(raw) < 162 {
		t.Fatal("public Descriptor absent")
	}
	var profile [32]byte
	copy(profile[:], raw[130:162])
	if _, err := reachability.Verify(raw, public.Target, [32]byte{1}, profile, time.Now()); err != nil {
		t.Fatal(err)
	}
	type original struct {
		Device, Inode uint64
		Mode          fs.FileMode
		Digest        [32]byte
	}
	snapshot := func() map[string]original {
		t.Helper()
		result := map[string]original{}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				t.Fatal("unexpected Store symlink")
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			st := info.Sys().(*syscall.Stat_t)
			v := original{Device: uint64(st.Dev), Inode: st.Ino, Mode: info.Mode()}
			if info.Mode().IsRegular() {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				v.Digest = sha256.Sum256(raw)
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			result[relative] = v
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	if len(before) != 5 {
		t.Fatal("expected complete original Store inventory", len(before))
	}
	for _, name := range []string{".", ".ardents-reachability-store-v3", ".ardents-reachability-store-lock", "records", filepath.Join("records", hex.EncodeToString(public.Target[:]))} {
		if _, ok := before[name]; !ok {
			t.Fatal("required existing Store object absent", name)
		}
	}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, Network: [32]byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	returned, outcome, lookupErr := store.Lookup(public.Target, profile, time.Now())
	closeErr := store.Close()
	if lookupErr != nil || outcome != reachability.Accepted || !bytes.Equal(returned, raw) || closeErr != nil {
		t.Fatal("retained exact revision2 unavailable", outcome, lookupErr, closeErr)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("read-only reopen changed original Store bytes or inodes")
	}
	t.Log("independent original-UID Store reopen preserved exact revision2, original objects and all retained bytes")
}
