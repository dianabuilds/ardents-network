package process

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSourceInvocationHasSeparateClosedProgramArgumentsAndPlacement(t *testing.T) {
	expected := SourceExpected{InstallationRoot: "/var/lib/ardents-source-installation", Generation: strings.Repeat("a", 64)}
	policy := sourceInvocationPolicy(expected)
	const program = "/var/lib/ardents-source-installation/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64"
	if policy.program != program || policy.arguments != program+"\x00source\x00start-installed\x00/var/lib/ardents-source-installation/binding.json\x00" || policy.placement != "0::/system.slice/ardents-source.service\n" {
		t.Fatal("Source fixed native policy differs", policy)
	}
	endpoint := endpointInvocationPolicy(Expected{InstallationRoot: expected.InstallationRoot, Generation: expected.Generation})
	if endpoint.program == policy.program || endpoint.arguments == policy.arguments || endpoint.placement == policy.placement {
		t.Fatal("Source consumed Endpoint invocation policy")
	}
}

func TestSourceNamespaceMechanismRetainsOwnKernelObjects(t *testing.T) {
	// The disposable test process exercises namespace custody only; no Source
	// factory, installed invocation, manager or accepting process proof is used.
	root, err := os.OpenRoot("/proc/self")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pin := &Invocation{root: root, pid: uint32(os.Getpid())}
	defer pin.Close()
	if err := pin.retainNamespaces(ctx); err != nil {
		t.Fatal(err)
	}
	if len(pin.namespaces) != 7 {
		t.Fatal("incomplete Source namespace inventory")
	}
	if err := pin.observeNamespaces(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pin.namespaces["mnt"].Close(); err != nil {
		t.Fatal(err)
	}
	if err := pin.observeNamespaces(ctx); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original namespace descriptor loss was accepted: %v", err)
	}
	cancel()
	if err := pin.observeNamespaces(ctx); err != context.Canceled {
		t.Fatalf("original cancellation did not deny namespace observation: %v", err)
	}
	if err := pin.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("physical namespace Close failure lost: %v", err)
	}
}

func TestSourceInvocationRefusesForeignPrincipalAndOriginalCancellation(t *testing.T) {
	for _, account := range [][2]uint32{{0, 0}, {99, 99}, {1000, 1000}, {100, 101}} {
		pin, err := retainSource(t.Context(), SourceExpected{UID: account[0], GID: account[1]})
		if pin != nil || !errors.Is(err, errInput) {
			t.Fatal("foreign Source principal reached native acquisition", account, err)
		}
	}
	info, err := os.Stat("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	pin, err := retainSource(ctx, SourceExpected{PID: uint32(os.Getpid()), UID: 100, GID: 100,
		Invocation: [16]byte{1}, InstallationRoot: "/var/lib/ardents-source-installation",
		Generation: strings.Repeat("a", 64), ProgramIdentity: info})
	if pin != nil || err != context.Canceled {
		t.Fatal("cancelled original reached Source process acquisition", err)
	}
	// This test executable is not an installed Source. No actual Source Start
	// or successful installed process fact is supplied by this negative control.
	pin, err = retainSource(t.Context(), SourceExpected{PID: uint32(os.Getpid()), UID: 100, GID: 100,
		Invocation: [16]byte{1}, InstallationRoot: "/var/lib/ardents-source-installation",
		Generation: strings.Repeat("a", 64), ProgramIdentity: info})
	if pin != nil || !errors.Is(err, errBinding) {
		_ = pin.Close()
		t.Fatal("ordinary test invocation impersonated installed Source", err)
	}
}
