package recoverysmoke

import (
	"strings"
	"testing"
	"time"

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

func TestStatsSamplerLeavesSampleCountToItsOwner(t *testing.T) {
	done := make(chan struct{})
	close(done)
	sampler := &statsSampler{cancel: func() {}, done: done, samples: []recovery.ResourceSample{
		{ClientRSS: 1, PublisherRSS: 1},
	}}
	samples, err := sampler.stop()
	if err != nil || len(samples) != 1 {
		t.Fatalf("samples=%d err=%v", len(samples), err)
	}
}

func TestResourceObservationCoalescesOnlyExactDockerRedraw(t *testing.T) {
	left := recovery.ResourceSample{AtNanos: 1, ClientRSS: 2, PublisherRSS: 3,
		ClientCPUPercent: 4, PublisherCPUPercent: 5, ClientReceived: 6, ClientSent: 7,
		PublisherReceived: 8, PublisherSent: 9}
	right := left
	right.AtNanos = int64(500 * time.Millisecond)
	if !sameResourceObservation(left, right) {
		t.Fatal("timestamp-only Docker redraw was not coalesced")
	}
	right.AtNanos = int64(time.Second)
	if sameResourceObservation(left, right) {
		t.Fatal("genuine unchanged resource interval was coalesced")
	}
	right.AtNanos = int64(500 * time.Millisecond)
	right.ClientReceived++
	if sameResourceObservation(left, right) {
		t.Fatal("changed resource observation was coalesced")
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
	endpointID := strings.Repeat("c", 64)
	identities["client-endpoint"] = endpointID
	service, err = addResourceRow([]byte(`{"ID":"`+endpointID[:12]+`","MemUsage":"2MiB / 32MiB",`+
		`"CPUPerc":"1%","NetIO":"9kB / 10kB"}`), identities,
		[]string{"client", "client-endpoint", "publisher"}, &sample)
	if err != nil || service != "client-endpoint" || sample.ClientRSS != 12<<20 ||
		sample.ClientReceived != 3000 || sample.ClientSent != 4000 {
		t.Fatalf("endpoint resources changed exact Route traffic: service=%q sample=%+v err=%v", service, sample, err)
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

func TestResourceRowAcceptsOnlyDockerStatsCursorFraming(t *testing.T) {
	identity := strings.Repeat("a", 64)
	row := `{"ID":"` + identity[:12] + `","MemUsage":"1MiB / 2MiB","CPUPerc":"1%","NetIO":"1kB / 1kB"}`
	for name, framed := range map[string]string{
		"cursor home": "\x1b[H" + row,
		"refresh":     "\x1b[J\x1b[H" + row + " \x1b[K",
	} {
		t.Run(name, func(t *testing.T) {
			service, err := addResourceRow([]byte(framed), map[string]string{"client": identity},
				[]string{"client"}, &recovery.ResourceSample{})
			if err != nil || service != "client" {
				t.Fatalf("service=%q err=%v", service, err)
			}
		})
	}
	if service, err := addResourceRow([]byte(" \x1b[K"), map[string]string{"client": identity},
		[]string{"client"}, &recovery.ResourceSample{}); err != nil || service != "" {
		t.Fatalf("control-only row service=%q err=%v", service, err)
	}
	for name, framed := range map[string]string{"text": "prefix" + row, "wide CSI": "\x1b[2J" + row,
		"suffix": row + " garbage"} {
		t.Run(name, func(t *testing.T) {
			if _, err := addResourceRow([]byte(framed), map[string]string{"client": identity},
				[]string{"client"}, &recovery.ResourceSample{}); err == nil {
				t.Fatal("unrecognized framing passed")
			}
		})
	}
}
