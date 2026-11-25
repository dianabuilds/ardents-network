//go:build darwin

package hosting

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The macOS adapter reads the full 64-bit IFMIB interface-data totals (#872),
// not the routing-socket if_data64 byte counters that wrap at 2^32 (#865). These
// tests prove the continuity guarantee at runtime on macOS: per-boot
// per-interface monotonic byte totals in both directions, a stable name->index
// binding, and a boot identity that stays equal within a boot.
func TestDarwinMeasureLoopbackContinuity(t *testing.T) {
	before, err := measureHosting([]string{"lo0"})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	sender, err := net.Dial("udp4", listener.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	if err := sender.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := sender.Write(make([]byte, 512)); err != nil {
		t.Fatal(err)
	}
	if count, _, err := listener.ReadFrom(make([]byte, 1024)); err != nil || count != 512 {
		t.Fatalf("loopback control: %d / %v", count, err)
	}
	after, err := measureHosting([]string{"lo0"})
	if err != nil {
		t.Fatal(err)
	}
	if before.Boot == "" || before.Boot != after.Boot ||
		before.Interfaces[0].Index != after.Interfaces[0].Index ||
		after.Interfaces[0].Tx-before.Interfaces[0].Tx < 512 ||
		after.Interfaces[0].Rx-before.Interfaces[0].Rx < 512 {
		t.Fatalf("native counters lost continuity: %+v -> %+v", before, after)
	}
}

// TestDarwinMeasureFullWidthHostTotals brackets the adapter reading between two
// netstat -ib link totals. netstat reads the same full 64-bit IFMIB source, so the
// adapter must track the true host total, not a value truncated mod 2^32. On a
// host whose loopback total exceeds 2^32 a routing-socket (wrapping) adapter would
// fall far below the bracket and fail; the bracket also tolerates ordinary traffic
// growth between the three reads.
func TestDarwinMeasureFullWidthHostTotals(t *testing.T) {
	beforeIn, beforeOut, err := netstatLinkBytes("lo0")
	if err != nil {
		t.Fatal(err)
	}
	reading, err := measureHosting([]string{"lo0"})
	if err != nil {
		t.Fatal(err)
	}
	afterIn, afterOut, err := netstatLinkBytes("lo0")
	if err != nil {
		t.Fatal(err)
	}
	iface, err := net.InterfaceByName("lo0")
	if err != nil {
		t.Fatal(err)
	}
	got := reading.Interfaces[0]
	if got.Name != "lo0" || got.Index != uint64(iface.Index) {
		t.Fatalf("name->index binding = %+v, want lo0 index %d", got, iface.Index)
	}
	if got.Rx < beforeIn || got.Rx > afterIn || got.Tx < beforeOut || got.Tx > afterOut {
		t.Fatalf("IFMIB totals outside netstat bracket: rx %d not in [%d,%d], tx %d not in [%d,%d]",
			got.Rx, beforeIn, afterIn, got.Tx, beforeOut, afterOut)
	}
	if afterIn > 1<<32 && got.Rx <= 1<<32 {
		t.Fatalf("receive counter wrapped at 2^32: host total %d, adapter %d", afterIn, got.Rx)
	}
}

// netstatLinkBytes returns the link-level input/output byte totals for one
// interface from netstat -ib -n. The trailing seven columns are
// Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll; the optional Address column shifts
// them, so the counters are read from the right of the <Link#> row.
func netstatLinkBytes(name string) (ibytes, obytes uint64, err error) {
	out, err := exec.Command("netstat", "-ib", "-n").Output()
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[0] != name || !strings.HasPrefix(fields[2], "<Link#") {
			continue
		}
		ibytes, err = strconv.ParseUint(fields[len(fields)-5], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		obytes, err = strconv.ParseUint(fields[len(fields)-2], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		return ibytes, obytes, nil
	}
	return 0, 0, fmt.Errorf("netstat has no link row for %s", name)
}
