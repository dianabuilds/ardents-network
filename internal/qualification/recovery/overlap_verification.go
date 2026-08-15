package recovery

import "time"

func verifyOverlapEvidence(cell replacementCell, scope hostScopeEvidence, reserved map[string]bool) Result {
	value := cell.Overlap
	if len(cell.Events) != 1 {
		return invalid("S4.3 overlap event cardinality is invalid")
	}
	event := cell.Events[0]
	observer := value.Observer
	if observer.ContainerID == "" || observer.ImageID == "" || observer.NetworkMode == "" || observer.User != "0:0" ||
		observer.PIDMode != "" || observer.IPCMode != "private" || !observer.ReadOnly || observer.Privileged ||
		!observer.Removed || observer.MountCount != 0 || observer.PidsLimit != 16 || observer.MemoryLimit != 32<<20 ||
		observer.NanoCPUs != 250_000_000 || reserved[observer.ContainerID] {
		return invalid("S4.3 overlap observer confinement is invalid")
	}
	if value.SocketDigest == "" || value.LocalAddress == "" || value.RemoteAddress != "172.31.20.16:4606" ||
		value.InterfaceName == "" || value.Inode == 0 || value.InterfaceIndex <= 0 ||
		value.ObservedAtNanos < event.FaultAtNanos || value.FaultAtNanos < value.ObservedAtNanos ||
		value.FaultAtNanos-event.FaultAtNanos > int64(time.Second) || value.FaultCompletedNanos < value.FaultAtNanos ||
		value.CarrierCutAfterNanos <= 0 || value.AbsenceAfterNanos < value.CarrierCutAfterNanos ||
		!value.Absent || !value.ObserverRemoved || value.FaultCompletedNanos >= event.CanaryNanos {
		return fail("S4.3 second fault was not externally observed inside the overlapping episode")
	}
	baseline := cell.BaselineFinalTraffic
	if !terminalTrafficSample(baseline, cell.BaselineTerminalNanos) {
		return invalid("S4.3 overlap paired baseline is incomplete")
	}
	for _, pair := range [][2]uint64{
		{cell.FinalTraffic.ClientReceived + cell.FinalTraffic.ClientSent,
			baseline.ClientReceived + baseline.ClientSent},
		{cell.FinalTraffic.PublisherReceived + cell.FinalTraffic.PublisherSent,
			baseline.PublisherReceived + baseline.PublisherSent},
	} {
		if trafficExcess(pair[0], pair[1]) > recoveryTrafficAllowance {
			return fail("S4.3 overlap exceeded the paired recovery traffic allowance")
		}
	}
	return Result{Verdict: "pass"}
}
