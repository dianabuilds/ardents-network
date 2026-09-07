//go:build ignore

// Disposable arithmetic model. It makes no measured latency or anonymity claim.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type stage struct {
	Name    string  `json:"name"`
	ColdRTT float64 `json:"cold_link_rtt_units"`
	WarmRTT float64 `json:"warm_link_rtt_units"`
}
type result struct {
	Carrier           string  `json:"carrier"`
	LinkRTTMS         float64 `json:"link_rtt_ms"`
	Stages            []stage `json:"stages"`
	ColdMS            float64 `json:"cold_ms"`
	WarmMS            float64 `json:"warm_ms"`
	ReferenceEnvelope bool    `json:"reference_envelope"`
	FitsAssumedBudget bool    `json:"fits_assumed_budget"`
}

func main() {
	// TCP requires one connect RTT before TLS; QUIC establishes its carrier
	// in one RTT. Fresh inner TLS, admission and Service work remain in both.
	results := make([]result, 0, 8)
	for _, carrier := range []string{"tcp-tls", "quic-v1"} {
		adjacent := 2.0
		if carrier == "quic-v1" {
			adjacent = 1
		}
		stages := []stage{
			{"entry TLS and bootstrap admission", adjacent + 1, 0},
			{"interior carrier, inner TLS and bootstrap", adjacent + 4, 0},
			{"issuer carrier and inner TLS", adjacent + 3, 0},
			{"two sequential issuer batches", 6, 0},
			{"admit the prepared forwarding prefix", 2, 0},
			{"descriptor carrier, inner TLS and lookup", adjacent + 6, 0},
			{"parallel terminal TLS; joins and capsule proceed concurrently", adjacent + 3, 3},
			{"capsule submission and publisher delivery", 3, 3},
			{"publisher terminal carrier, inner TLS and join", adjacent + 6, 6},
			{"end-to-end Service TLS", 6, 6},
			{"coalesced Instance authentication and initial Continuity", 6, 6},
			{"application request and complete response", 6, 6},
			{"terminal receipt and peer confirmation", 6, 6},
		}
		for _, rtt := range []float64{20, 40, 80, 120} {
			v := result{Carrier: carrier, LinkRTTMS: rtt, Stages: stages, ReferenceEnvelope: rtt == 20}
			// Conservative total endpoint transfer reservations, including control,
			// handshakes and the 64 KiB response: cold 240 KiB at 20 Mbit/s,
			// warm 128 KiB at 100 Mbit/s. These are assumptions to falsify, not captures.
			v.ColdMS = (240*1024*8/20000000.0)*1000 + 500 + 250
			v.WarmMS = (128*1024*8/100000000.0)*1000 + 150 + 50
			for _, s := range stages {
				v.ColdMS += s.ColdRTT * rtt
				v.WarmMS += s.WarmRTT * rtt
			}
			v.FitsAssumedBudget = v.ColdMS <= 3000 && v.WarmMS <= 1000
			results = append(results, v)
			if v.ReferenceEnvelope && !v.FitsAssumedBudget {
				fmt.Fprintln(os.Stderr, "selected reference does not fit its predeclared model")
				os.Exit(1)
			}
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
