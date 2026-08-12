package node

import "sort"

func nearestRank95(values []uint64) uint64 {
	ordered := append([]uint64(nil), values...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	index := (95*len(ordered)+99)/100 - 1
	return ordered[index]
}
