//go:build browsercompat

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateTransitFaultConfigRejectsAmbiguousOrUnboundedFaults(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "transit-fault-ready")
	valid := config{
		TransparentApplication:      true,
		TransitFault:                transitFaultCarrierLoss,
		TransitFaultReadyPath:       ready,
		CarrierRelayListenAddress:   "203.0.113.10:49100",
		CarrierRelayTargetAddress:   "127.0.0.1:49100",
		CarrierRelayReadyPath:       filepath.Join(t.TempDir(), "relay-ready"),
		CarrierRelayResetPath:       filepath.Join(t.TempDir(), "relay-reset"),
		CarrierRelayResetResultPath: filepath.Join(t.TempDir(), "relay-reset-result"),
		DynamicWorkload: dynamicWorkloadConfig{Cycles: 60, IntervalMilliseconds: 1_000,
			CycleDeadlineMilliseconds: 5_000, NoFallbackEvery: 60, BytesEachDirection: 4 << 20},
	}
	if err := validateTransitFaultConfig(valid); err != nil {
		t.Fatalf("valid Carrier fault was rejected: %v", err)
	}
	valid.TransitFault = transitFaultProductNodeLoss
	if err := validateTransitFaultConfig(valid); err != nil {
		t.Fatalf("valid product Node fault was rejected: %v", err)
	}
	for name, mutate := range map[string]func(*config){
		"unknown":            func(input *config) { input.TransitFault = "transport-loss" },
		"no-ready":           func(input *config) { input.TransitFaultReadyPath = "" },
		"relative-ready":     func(input *config) { input.TransitFaultReadyPath = "relative" },
		"non-transparent":    func(input *config) { input.TransparentApplication = false },
		"publisher-terminal": func(input *config) { input.PublisherTerminal = publisherTerminalEndpointStop },
		"no-workload":        func(input *config) { input.DynamicWorkload = dynamicWorkloadConfig{} },
		"no-relay":           func(input *config) { input.CarrierRelayListenAddress = "" },
	} {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if err := validateTransitFaultConfig(input); err == nil {
				t.Fatal("invalid transit fault config was accepted")
			}
		})
	}
	withoutFault := config{}
	if err := validateTransitFaultConfig(withoutFault); err != nil {
		t.Fatalf("omitted transit fault changed ordinary fixture behavior: %v", err)
	}
	withoutFault.TransitFaultReadyPath = ready
	if err := validateTransitFaultConfig(withoutFault); err == nil {
		t.Fatal("orphan transit fault ready path was accepted")
	}
}

func TestConfiguredDynamicCarrierLossFollowsWarmupWithoutReconnect(t *testing.T) {
	testConfiguredDynamicTransitFault(t, transitFaultCarrierLoss, "Carrier loss closed the local Application handoff")
}

func TestConfiguredDynamicProductNodeLossFollowsWarmupWithoutReconnect(t *testing.T) {
	testConfiguredDynamicTransitFault(t, transitFaultProductNodeLoss, "product Node loss closed the local Application handoff")
}

func testConfiguredDynamicTransitFault(t *testing.T, fault transitFault, terminal string) {
	t.Helper()
	root := t.TempDir()
	proof, ready := filepath.Join(root, "proof"), filepath.Join(root, "transit-fault-ready")
	plan := dynamicWorkloadPlan{cycles: 2, interval: 50 * time.Millisecond, cycleDeadline: time.Second}
	client, accept := dynamicWorkloadTestClient(t)
	server := make(chan dynamicWorkloadServerResult, 1)
	closerContext, cancelCloser := context.WithCancel(t.Context())
	defer cancelCloser()
	go func() {
		accepted, err := accept()
		if err != nil {
			server <- dynamicWorkloadServerResult{err: err}
			return
		}
		closerDone := make(chan struct{})
		go closeDynamicWorkloadEndpointAtReady(closerContext, accepted, ready, closerDone)
		workload, serveErr := serveConfiguredDynamic(accepted, proof, dynamicWorkloadControls{transitFaultReady: ready}, plan, "", fault)
		cancelCloser()
		<-closerDone
		server <- dynamicWorkloadServerResult{workload: workload, err: serveErr}
	}()

	workload, err := exerciseConfiguredDynamic(client, "http://reference.ard/", plan, "", fault)
	if err != nil {
		t.Fatal(err)
	}
	result := <-server
	if result.err == nil || !strings.Contains(result.err.Error(), terminal) {
		t.Fatalf("dynamic %s terminal = %v", fault, result.err)
	}
	if workload.CompletedCycles != 2 || !workload.TerminalNoFallback || result.workload.CompletedCycles != 2 {
		t.Fatalf("dynamic %s progress = user %+v / server %+v", fault, workload, result.workload)
	}
	if contents, err := os.ReadFile(ready); err != nil || string(contents) != "ready\n" {
		t.Fatalf("dynamic %s readiness = %q / %v", fault, contents, err)
	}
	if contents, err := os.ReadFile(proof); err != nil || string(contents) != "dynamic-"+string(fault)+"\n" {
		t.Fatalf("dynamic %s proof = %q / %v", fault, contents, err)
	}
}
