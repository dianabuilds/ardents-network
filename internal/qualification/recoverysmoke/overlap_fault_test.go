package recoverysmoke

import (
	"context"
	"testing"
)

func TestOverlapControllerFaultsOnlyOneObservedReplacementCarrier(t *testing.T) {
	calls := 0
	value := carrierObservation{SocketID: "socket", SocketIDSHA256: "digest", LocalAddress: "172.31.20.24:4000",
		RemoteAddress: "172.31.20.16:4606", Inode: 7, InterfaceName: "ardents-test-absent", InterfaceIndex: 2}
	receipt, err := waitAndFaultOverlap(context.Background(), value.RemoteAddress,
		func(string) ([]carrierObservation, error) {
			calls++
			if calls == 1 {
				return nil, nil
			}
			return []carrierObservation{value}, nil
		}, func(name string) error { return nil })
	if err != nil || receipt.Kind != "overlap-faulted" || receipt.Observation != value || !receipt.Absent {
		t.Fatalf("overlap receipt=%+v err=%v", receipt, err)
	}
}
