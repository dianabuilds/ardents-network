package nodelifecycle

import (
	"errors"
	"runtime"
	"runtime/metrics"
)

const maximumGoMemory = 320 << 20

func checkProcessPlacement() error {
	if runtime.GOMAXPROCS(0) != 1 {
		return errors.New("H3-NP1 requires GOMAXPROCS=1")
	}
	samples := []metrics.Sample{{Name: "/gc/gomemlimit:bytes"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 || samples[0].Value.Uint64() > maximumGoMemory {
		return errors.New("H3-NP1 requires a 320 MiB Go memory limit")
	}
	return nil
}
