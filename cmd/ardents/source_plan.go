package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

type sourcePlan struct {
	Schema               string             `json:"schema"`
	NetworkID            string             `json:"network_id"`
	AuthorityPublic      []string           `json:"authority_public"`
	Threshold            int                `json:"threshold"`
	ClockObservedAt      string             `json:"clock_observed_at"`
	ClockObservationFile string             `json:"clock_observation_file,omitempty"`
	OrderSeed            string             `json:"order_seed"`
	MaterializationIndex uint32             `json:"materialization_index"`
	RefreshIntervalMS    uint32             `json:"refresh_interval_ms,omitempty"`
	RuntimeProfile       string             `json:"runtime_profile,omitempty"`
	ClientCertificate    string             `json:"client_certificate"`
	ClientKey            string             `json:"client_key"`
	Sources              []sourcePlanMember `json:"sources"`
}

type sourcePlanMember struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

func runRefreshSources(ctx context.Context, arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("refresh-sources", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root, planPath, once, resume := "", "", false, false
	flags.StringVar(&root, "state-root", "", "owned state root")
	flags.StringVar(&planPath, "source-plan", "", "bounded source plan JSON")
	flags.BoolVar(&once, "once", false, "perform exactly one source wave")
	flags.BoolVar(&resume, "resume", false, "resume automatic refresh from current state")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return errors.New("usage: ardents refresh-sources --state-root PATH --source-plan PATH")
	}
	events := newEventOutput(output)
	config, err := readSourcePlan(root, planPath)
	if err != nil {
		return err
	}
	config.ObserveResources = events.append
	store, err := state.Open(config)
	if err != nil {
		return fmt.Errorf("open network state: %w", err)
	}
	defer store.Close()
	snapshot, err := store.Current()
	if !resume {
		snapshot, err = store.Refresh(ctx)
	}
	if err != nil {
		return fmt.Errorf("refresh network state: %w", err)
	}
	err = events.encode(struct {
		Schema             string    `json:"schema"`
		Kind               string    `json:"kind"`
		Generation         string    `json:"generation"`
		Epoch              uint64    `json:"epoch"`
		SourceAttempts     uint16    `json:"source_attempts"`
		SourceOutcomes     [4]string `json:"source_outcomes"`
		LatestCompleteness string    `json:"latest_completeness"`
	}{"ardents-h3-source-event-v1", "source-wave-accepted", snapshot.Generation, snapshot.Epoch,
		snapshot.SourceAttempts, snapshot.SourceOutcomes, snapshot.LatestCompleteness})
	if err != nil || once || config.AutomaticRefreshInterval == 0 {
		return err
	}
	return store.Wait(ctx)
}

func readSourcePlan(root, path string) (state.Config, error) {
	var plan sourcePlan
	if err := planfile.Decode(path, 32<<10, &plan); err != nil {
		return state.Config{}, fmt.Errorf("decode source plan: %w", err)
	}
	if plan.Schema != "ardents-h3-source-plan-v1" || len(plan.Sources) != 2 {
		return state.Config{}, errors.New("source plan is not canonical or complete")
	}
	var err error
	config := state.Config{Root: root, Threshold: plan.Threshold,
		Source: source.Config{MaterialIndex: plan.MaterializationIndex}, RuntimeProfile: plan.RuntimeProfile,
		AutomaticRefreshInterval: time.Duration(plan.RefreshIntervalMS) * time.Millisecond}
	if err := planfile.FixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return config, err
	}
	config.Authorities, err = planfile.Authorities(plan.AuthorityPublic, 16)
	if err != nil {
		return config, err
	}
	config.Clock = time.Now
	config.ClockObservationFile = plan.ClockObservationFile
	if config.ClockObservation, err = time.Parse(time.RFC3339, plan.ClockObservedAt); err != nil {
		return config, err
	}
	if err := planfile.FixedHex(plan.OrderSeed, config.Source.OrderSeed[:]); err != nil {
		return config, err
	}
	return config, loadSourceCredentials(&config, plan)
}
