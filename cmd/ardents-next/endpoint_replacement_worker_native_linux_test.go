//go:build linux && endpoint_launch_native && text_worker_installed

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// TestNativeEndpointReplacementWithPublisher joins an already admitted Worker
// owning an actual publication before staging the next signed Release.
func TestNativeEndpointReplacementWithPublisher(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Endpoint replacement qualification requires a root driver")
	}
	f, _ := installedServiceNetwork(t, string(transport.ClosedCarrierTCP))
	stand := provisionPlatformLedLaunchFixtureWithSuccessor(t, 1000, f, true)
	credential := &syscall.Credential{Uid: stand.uid, Gid: stand.gid, Groups: []uint32{}}
	installedServiceInstance(t, stand.program, credential, stand.base, stand.network.NetworkID)
	verifyNativeEndpointReplacement(t, stand, func(t *testing.T, original [3]string) string {
		ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
		defer cancel()
		if _, err := administration.RequestSnapshot(ctx, stand.administrationSock, []byte("Endpoint replacement snapshot\n")); err != nil {
			t.Fatal("publish before replacement:", err)
		}
		if link, err := administration.RequestPublishedLink(ctx, stand.administrationSock); err != nil || link == "" {
			t.Fatal("publication was not reachable before replacement:", err)
		}
		const directory = "/sys/fs/cgroup/system.slice"
		scopes, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		suffix := "-" + original[0] + "-" + strconv.FormatUint(uint64(stand.uid), 10) + ".service"
		for _, scope := range scopes {
			name := scope.Name()
			if scope.IsDir() && strings.HasPrefix(name, "ardents-text-publisher@") && strings.HasSuffix(name, suffix) {
				path := filepath.Join(directory, name)
				if liveSnapshotWorker(path) {
					return path
				}
			}
		}
		t.Fatal("published snapshot has no original installed Publisher worker")
		return ""
	})
}
