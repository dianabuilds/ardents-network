package process

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestForwardingInvocationKeepsSeparateFixedPurpose(t *testing.T) {
	e := ForwardingExpected{InstallationRoot: "/var/lib/ardents-node-installation", Generation: strings.Repeat("a", 64)}
	p := forwardingInvocationPolicy(e)
	const program = "/var/lib/ardents-node-installation/generations/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/ardents-node-linux-amd64"
	if p.program != program || p.arguments != program+"\x00forwarding\x00start-installed\x00/var/lib/ardents-node-installation/binding.json\x00" || p.placement != "0::/system.slice/ardents-forwarding.service\n" {
		t.Fatal("Forwarding did not select its exact closed native policy")
	}
	s := sourceInvocationPolicy(SourceExpected{InstallationRoot: e.InstallationRoot, Generation: e.Generation})
	ep := endpointInvocationPolicy(Expected{InstallationRoot: e.InstallationRoot, Generation: e.Generation})
	if p.arguments == s.arguments || p.arguments == ep.arguments || p.placement == s.placement || p.placement == ep.placement {
		t.Fatal("another invocation policy authorized Forwarding")
	}
}

func TestForwardingStopArchiveFactsCannotCreateInvocationOrSignalRight(t *testing.T) {
	for _, ids := range [][2]uint64{{0, 1}, {1, 0}, {0, 0}} {
		p, err := RetainForwardingStop(t.Context(), ForwardingExpected{}, ids[0], ids[1])
		if !errors.Is(err, errInput) || p.original != nil {
			t.Fatal("missing original native identity supplied an invocation", err)
		}
		if attempted, err := p.InterruptForwarding(t.Context()); attempted || err == nil {
			t.Fatal("detached archive facts supplied a signal right", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	p, err := RetainForwardingStop(ctx, ForwardingExpected{PID: uint32(os.Getpid()), UID: 988, GID: 988, InstallationRoot: "/var/lib/unopened-forwarding", Generation: strings.Repeat("a", 64)}, 1, 1)
	if err == nil || p.original != nil {
		t.Fatal("ordinary process/canceled caller impersonated archived Forwarding", err)
	}
}

func TestForwardingInvocationRefusesImpersonationAndCancelledAcquisition(t *testing.T) {
	for _, ids := range [][2]uint32{{0, 0}, {99, 99}, {1000, 1000}, {100, 101}} {
		p, err := retainForwarding(t.Context(), ForwardingExpected{UID: ids[0], GID: ids[1]})
		if p.original != nil || !errors.Is(err, errInput) {
			t.Fatal("foreign principal reached native process acquisition", ids, err)
		}
	}
	info, err := os.Stat("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	e := ForwardingExpected{PID: uint32(os.Getpid()), UID: 100, GID: 100,
		Invocation: [16]byte{1}, InstallationRoot: "/var/lib/ardents-node-installation",
		Generation: strings.Repeat("a", 64), ProgramIdentity: info}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	p, err := retainForwarding(ctx, e)
	if p.original != nil || err != context.Canceled {
		t.Fatal("cancelled acquisition returned an observation", err)
	}
	// The real test executable must refuse. It does not impersonate a successful
	// installed process or supply any Startup, manager or domain authority.
	p, err = retainForwarding(t.Context(), e)
	if p.original != nil || !errors.Is(err, errBinding) {
		_ = p.Close()
		t.Fatal("ordinary test executable impersonated installed Forwarding", err)
	}
	p, err = RetainForwardingSelf(t.Context(), e)
	if p.original != nil || !errors.Is(err, errInput) {
		_ = p.Close()
		t.Fatal("foreign self credentials accepted", err)
	}
}

func TestForwardingPhysicalCloseRetainsOriginalFailureAcrossCopies(t *testing.T) {
	// Exercise native descriptor retirement only, without a successful fixed
	// process admission. The real proc root and namespace handles are owned here.
	r, err := os.OpenRoot("/proc/self")
	if err != nil {
		t.Fatal(err)
	}
	pin := &Invocation{root: r, pid: uint32(os.Getpid())}
	pin.identity, err = r.Stat(".")
	if err != nil {
		_ = pin.Close()
		t.Fatal(err)
	}
	if err := pin.retainNamespaces(t.Context()); err != nil {
		_ = pin.Close()
		t.Fatal(err)
	}
	original := &forwardingInvocation{pin: pin}
	p := ForwardingInvocation{original: original}
	defer p.Close()
	copy := p
	if err := pin.namespaces["mnt"].Close(); err != nil {
		t.Fatal(err)
	}
	err = p.Observe(t.Context())
	if !errors.Is(err, os.ErrClosed) || !errors.Is(err, errBinding) {
		t.Fatalf("original physical loss did not deny observation: %v", err)
	}
	if err := copy.Observe(t.Context()); !errors.Is(err, os.ErrClosed) || !errors.Is(err, errBinding) {
		t.Fatalf("copy revived original failure: %v", err)
	}
	if attempted, err := p.InterruptForwarding(t.Context()); attempted || err == nil {
		t.Fatal("failed non-authoritative pin acquired a signal right", err)
	}
	closed := copy.Close()
	if !errors.Is(closed, os.ErrClosed) || !errors.Is(closed, errBinding) {
		t.Fatalf("Close erased original failure or physical close failure: %v", closed)
	}
	if err := p.Close(); err != closed {
		t.Fatal("copied Close did not retain the exact original result")
	}
}
