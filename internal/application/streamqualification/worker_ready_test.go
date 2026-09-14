package streamqualification

import "testing"

func TestQualificationWorkloadWaitsForFullOpenedCreditedSet(t *testing.T) {
	schedule, err := ClientToPublisher.Definition(ReaderRole)
	if err != nil {
		t.Fatal(err)
	}
	streams := make(map[uint32]*workerStream, schedule.OpenConnections)
	order := make([]uint32, 0, schedule.OpenConnections)
	for index := uint16(0); index < schedule.OpenConnections; index++ {
		id := uint32(index)*2 + 1
		streams[id] = &workerStream{id: id, opened: true}
		order = append(order, id)
	}
	if qualificationWorkloadReady(streams, order, schedule) {
		t.Fatal("workload started before active credit")
	}
	for index := 0; index < int(schedule.ActiveConnections); index++ {
		streams[order[index]].sendCredit = frameCreditWindow
	}
	if !qualificationWorkloadReady(streams, order, schedule) {
		t.Fatal("workload was not ready after full open and active credit")
	}
	if qualificationWorkloadReady(streams, order[:len(order)-1], schedule) {
		t.Fatal("workload started before full connection set")
	}
}
