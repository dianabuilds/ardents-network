//go:build darwin

package counters

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// macOS measurement reads the IFMIB interface-data sysctl
// (net.link.generic.ifdata.<ifindex>, IFDATA_GENERAL), which reports full 64-bit
// per-interface byte totals. The routing-socket if_data64 byte counters
// (NET_RT_IFLIST2 and the classic NET_RT_IFLIST) are truncated to 32 bits on
// macOS: they wrap at 2^32 without a boot or interface-index change, so they
// cannot preserve the per-boot per-interface monotonic byte continuity that
// hostingState.observe requires. This adapter therefore reads IFMIB, never the
// routing-socket counters (#865 finding, #872 redesign).
//
// The IFDATA_GENERAL selector and the ifmibdata field offsets below are not
// exported by the admitted x/sys/unix and are admitted here from the macOS SDK
// net/if_mib.h and net/if_var.h, verified by an offsetof probe and an exact
// netstat -ib runtime match. struct ifmibdata is ifmd_name[IFNAMSIZ] followed by
// ifmd_pcount, ifmd_flags, ifmd_snd_len, ifmd_snd_maxlen, ifmd_snd_drops (five
// uint32) and ifmd_filler[4] (four uint32) = 52 bytes, then ifmd_data (struct
// if_data64, which the kernel aligns to 4 bytes) at offset 52. Within if_data64
// ifi_ibytes is at +64 and ifi_obytes at +72, and sizeof(struct ifmibdata) is 180.
// The named sysctl resolves the CTL_NET/PF_LINK/NETLINK_GENERIC/IFMIB_IFDATA MIB
// through the kernel, so only the selector and these offsets are owned here. The
// raw buffer is decoded at those offsets rather than overlaid, keeping the
// adapter free of unsafe.
const (
	ifdataGeneral     = 1                    // IFDATA_GENERAL: generic interface statistics.
	ifmibDataOffset   = 52                   // offsetof(struct ifmibdata, ifmd_data).
	ifmibIbytesOffset = ifmibDataOffset + 64 // ifi_ibytes within struct if_data64.
	ifmibObytesOffset = ifmibDataOffset + 72 // ifi_obytes within struct if_data64.
	ifmibSize         = 180                  // sizeof(struct ifmibdata).
)

func Supported() bool { return true }

// Read returns an observation of the named interfaces and original boot.
func Read(names []string) (Reading, error) {
	var reading Reading
	boot, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return reading, err
	}
	// Canonical boot identity: the kernel boot timeval is stable within a boot and
	// changes on reboot. observe() only requires a non-empty string that stays
	// equal across observations of the same boot.
	reading.Boot = fmt.Sprintf("%d.%06d", boot.Sec, boot.Usec)
	for _, name := range names {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			return Reading{}, err
		}
		index := iface.Index
		raw, err := unix.SysctlRaw("net.link.generic.ifdata", index, ifdataGeneral)
		if err != nil {
			return Reading{}, err
		}
		if len(raw) != ifmibSize {
			return Reading{}, errors.New("hosting interface counter is unavailable")
		}
		rx := binary.LittleEndian.Uint64(raw[ifmibIbytesOffset : ifmibIbytesOffset+8])
		tx := binary.LittleEndian.Uint64(raw[ifmibObytesOffset : ifmibObytesOffset+8])
		// Recheck identity after sampling: a removed/recreated device is no longer
		// the counter source bound to this period, even when its name is reused.
		after, err := net.InterfaceByName(name)
		if err != nil || after.Index != index {
			return Reading{}, errors.New("hosting interface changed during observation")
		}
		reading.Interfaces = append(reading.Interfaces, Interface{Name: name, Index: uint64(index), Tx: tx, Rx: rx})
	}
	return reading, nil
}
