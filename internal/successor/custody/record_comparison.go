package custody

import "bytes"

func floorEqualsState(floor authorityFloor, state authorityState) bool {
	return floor.Generation == state.Generation && floor.Revision == state.Revision && equalWatermarks(floor.Watermarks, state.Watermarks)
}
func sameAuthorityState(left, right authorityState) bool {
	return left.Binding == right.Binding && left.Generation == right.Generation && left.Revision == right.Revision && equalWatermarks(left.Watermarks, right.Watermarks) && bytes.Equal(left.RootMaterial, right.RootMaterial) && bytes.Equal(left.AdmissionJournal, right.AdmissionJournal)
}
