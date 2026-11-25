//go:build darwin

package hosting

// macOS measurement is staged unavailable (#865). The universal budget core
// derives per-boot per-interface monotonic byte continuity from the facts a
// measure adapter supplies (hostingState.observe): the same boot, the same
// name+index, and non-decreasing transmit/receive totals. A macOS adapter must
// supply facts that preserve that guarantee, never a weaker accepting path.
//
// Runtime evidence on macOS 27.0.1 (arm64) falsifies the NET_RT_IFLIST2 design
// premise that IfData64.Ibytes/Obytes are 64-bit monotonic host totals. The
// routing-socket if_data64 byte counters are truncated to 32 bits: for en0 the
// snapshot reported Ibytes=2646041600 while the true host total was
// 165855302598 (the snapshot equals the true total mod 2^32), and the same
// truncation holds for lo0 and for the classic NET_RT_IFLIST if_data counters.
// Packet counters are full-width; only the byte totals wrap. A byte counter that
// wraps at 2^32 resets without a boot or interface-index change, so observe()
// would refuse continuity after every 4 GiB on a busy interface. That cannot
// preserve the guarantee, so this adapter refuses rather than accept a weaker
// counter source.
//
// The full 64-bit totals are available through the IFMIB interface-data sysctl
// (net.link.generic.ifdata.<ifindex>, IFDATA_GENERAL): the same probe read
// 165861754137 there, an exact match to the host total. A future macOS measure
// port should resolve name->index and read that source (its constants/struct are
// not exported by the admitted x/sys/unix and need their own reviewed admission),
// with the same boot-identity and index-recheck continuity facts. Until that
// redesign is accepted, macOS receives the explicit typed refusal.
func hostingPlatform() error { return ErrUnsupportedPlatform }

func measureHosting([]string) (hostingReading, error) {
	return hostingReading{}, ErrUnsupportedPlatform
}
