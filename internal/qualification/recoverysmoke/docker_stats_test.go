package recoverysmoke

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func TestResourceSummaryUsesLiveHighWater(t *testing.T) {
	samples := []recovery.ResourceSample{
		{ClientRSS: 10, PublisherRSS: 20, ClientCPUPercent: 1, PublisherCPUPercent: 2,
			ClientReceived: 3, ClientSent: 4, PublisherReceived: 5, PublisherSent: 6},
		{ClientRSS: 30, PublisherRSS: 25, ClientCPUPercent: 4, PublisherCPUPercent: 3,
			ClientReceived: 7, ClientSent: 8, PublisherReceived: 9, PublisherSent: 10},
	}
	memory, cpu := resourceHighWater(samples)
	if memory != 30 || cpu != 4 {
		t.Fatalf("summary=%d %.1f", memory, cpu)
	}
}

func TestResourceRowBindsTheExactContainerSide(t *testing.T) {
	clientID, publisherID := strings.Repeat("a", 64), strings.Repeat("b", 64)
	identities := map[string]string{"client": clientID, "publisher": publisherID}
	services := []string{"client", "publisher"}
	var sample recovery.ResourceSample
	service, err := addResourceRow([]byte(`{"ID":"`+clientID[:12]+`","MemUsage":"10MiB / 32MiB",`+
		`"CPUPerc":"2%","NetIO":"3kB / 4kB"}`), identities, services, &sample)
	if err != nil || service != "client" || sample.ClientRSS != 10<<20 || sample.ClientReceived != 3000 ||
		sample.ClientSent != 4000 || sample.PublisherRSS != 0 {
		t.Fatalf("service=%q sample=%+v err=%v", service, sample, err)
	}
	for name, identity := range map[string]string{"empty": "", "short": "a", "mismatch": clientID[:12] + "f"} {
		t.Run(name, func(t *testing.T) {
			row := []byte(`{"ID":"` + identity + `","MemUsage":"1MiB / 2MiB","CPUPerc":"1%","NetIO":"1kB / 1kB"}`)
			if _, err := addResourceRow(row, identities, services, &recovery.ResourceSample{}); err == nil {
				t.Fatal("malformed identity passed")
			}
		})
	}
}
