//go:build linux && join_installed_native

package systemd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Real fixed manager queries, never fixture authority. A root-created alias is
// a refusal-only native canary; no foreign Node or accepting role is started.
func TestJoinSlotsNativeConcurrentQueriesMatchSequential(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-join-initial-native-stand")
	if err != nil || string(marker) != "ardents-join-initial-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: selected private genuine JOIN stand required", err)
	}
	const alias = "/etc/systemd/system/ardents-issuer.service"
	const target = "/usr/lib/systemd/system/dbus.service"
	if _, err := os.Lstat(alias); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canary slot not initially absent", err)
	}
	sequential := func(ctx context.Context, r *managerReference) error {
		for _, unit := range nodeUnitNames {
			if unit != "ardents-join.service" {
				if err := nodeSlotAbsent(ctx, r, unit); err != nil {
					return err
				}
			}
		}
		return nodeNameAbsent(ctx, r, "ardents-join.socket")
	}
	query := func(parallel bool) error {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		r, err := openManagerReference(ctx, joinUnitReply)
		if r == nil {
			return err
		}
		if err == nil {
			err = r.initializeManagerDeclaration()
		}
		if err == nil {
			err = r.client.observeRequests(ctx, func(ctx context.Context) error {
				if err := r.client.sameOwner(ctx, r.client.manager); err != nil {
					return err
				}
				var err error
				if parallel {
					err = joinOtherSlotsAbsent(ctx, r)
				} else {
					err = sequential(ctx, r)
				}
				if err != nil {
					return err
				}
				return r.client.sameOwner(ctx, r.client.manager)
			})
		}
		return errors.Join(err, r.close())
	}
	for _, parallel := range []bool{false, true} {
		if err := query(parallel); err != nil {
			t.Fatal("actual absent slots refused", parallel, err)
		}
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if original == nil {
			return
		}
		current, err := os.Lstat(alias)
		if err != nil || !os.SameFile(original, current) {
			t.Error("owned canary changed; preserve", err)
			return
		}
		if err := os.Remove(alias); err != nil {
			t.Error(err)
		}
	}()
	for _, parallel := range []bool{false, true} {
		if err := query(parallel); err == nil {
			t.Fatal("foreign alias passed fixed slot observation", parallel)
		}
	}
	// Original inode removal is cleanup of this negative canary only. A fresh
	// observer repeats every native request; no earlier positive result is used.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	// Disarm deferred removal using an explicit same-inode receipt.
	original = nil
	for _, parallel := range []bool{false, true} {
		if err := query(parallel); err != nil {
			t.Fatal("fresh absence after owned canary joined", parallel, err)
		}
	}
	// Actual fixed-unit replies measure conversion separately from native bus
	// I/O. The original root control never Starts; it joins before bootstrap.
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	control, err := OpenJoinControl(ctx)
	if control != nil {
		defer func() {
			if err := control.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	if err != nil {
		t.Fatal("actual fixed control for typed observation timing", err)
	}
	r := control.original.reference
	for _, iface := range []string{"Unit", "Service"} {
		began := time.Now()
		reply, err := r.client.call(ctx, "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice", "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
		if err != nil {
			t.Fatal(err)
		}
		native := time.Since(began)
		began = time.Now()
		count := 0
		for range 30 {
			properties, err := startupProperties(reply)
			if err != nil {
				t.Fatal(err)
			}
			count = len(properties)
		}
		t.Logf("actual %s GetAll: native=%s properties=%d; full typed conversion mean=%s", iface, native, count, time.Since(began)/30)
	}
	t.Log("sequential/concurrent actual slot observations agree on absent, foreign alias, and fresh absent namespace")
}
