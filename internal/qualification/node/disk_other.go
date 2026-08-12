//go:build !linux

package node

func runNodeDiskWrapper() Result {
	return Result{Verdict: "invalid", Reason: "disk-full node wrapper requires Linux"}
}
