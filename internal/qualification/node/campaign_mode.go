package node

import "time"

type nodeCampaignMode struct {
	duration     time.Duration
	sampleLimit  int
	sampleBudget int64
	short        bool
	churn        bool
	workload     []string
	faults       []string
}

func selectNodeCampaignMode(name string) (nodeCampaignMode, bool) {
	modes := map[string]nodeCampaignMode{
		"short": {sampleLimit: 4_096, sampleBudget: int64(512) << 20, short: true,
			workload: []string{"assignment-stabilization", "authenticated-probe", "restart-quiescence"},
			faults:   []string{"source-death", "clock-uncertainty", "memory-pressure", "cpu-pressure", "source-pressure", "evidence-failure", "emfile", "disk-full", "cgroup-drift"}},
		"churn-2h": {duration: 2 * time.Hour, sampleLimit: 8_000, sampleBudget: int64(2) << 30, churn: true,
			workload: []string{"assignment-stabilization", "probe-15m", "restart-5m", "quiescence-120s"},
			faults:   []string{"node-memory-pressure", "node-cpu-pressure", "h3-s-source-pressure"}},
		"unattended-24h": {duration: 24 * time.Hour, sampleLimit: 87_000, sampleBudget: int64(8) << 30,
			workload: []string{"assignment-stabilization", "health-30s", "probe-15m"}},
	}
	mode, found := modes[name]
	return mode, found
}
