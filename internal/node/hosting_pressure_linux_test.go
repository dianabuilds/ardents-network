//go:build linux

package node

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestHostingPressureUsesDeclaredOwnerMeasurement(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "sample", true: "failure"}[fail], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "hosting")
			now := time.Now().UTC().Truncate(time.Second)
			policy := resource.HostingPolicy{Provider: "component fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1,
				Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
			if err := resource.InitializeHosting(root, policy); err != nil {
				t.Fatal(err)
			}
			host, err := hosting.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := host.Close(); err != nil {
					t.Error(err)
				}
			})
			want := resource.Sample{RSSBytes: 17 << 20, CPUUsageUsec: 123, MemoryBytes: 19 << 20}
			cause := errors.New("fixture owner measurement unavailable")
			calls := 0
			config := runtimeConfig{host: host, now: time.Now, Config: Config{ResourceMeasure: func() (resource.Sample, error) {
				calls++
				if fail {
					return resource.Sample{}, cause
				}
				return want, nil
			}}}
			level, err := config.hostingPressure()
			if calls != 1 {
				t.Fatalf("declared owner measurement calls = %d, want 1", calls)
			}
			if fail {
				if level != pressureDrain || !errors.Is(err, cause) {
					t.Fatalf("failed measurement: level=%v err=%v", level, err)
				}
				return
			}
			if err != nil || level != pressureNormal || config.hostingUsage != want {
				t.Fatalf("owner evidence: level=%v usage=%+v err=%v", level, config.hostingUsage, err)
			}
		})
	}
}
