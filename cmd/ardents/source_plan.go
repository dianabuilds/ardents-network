package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/runtimeplan"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type sourcePlan = runtimeplan.Source
type sourcePlanMember = runtimeplan.SourceMember

func runRefreshSources(ctx context.Context, arguments []string, output io.Writer) (resultErr error) {
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
	if once && resume {
		return errors.New("refresh-sources --once and --resume are mutually exclusive")
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
	defer func() { resultErr = errors.Join(resultErr, store.Close()) }()
	if resume {
		// Resume only confirms the accepted State is readable and then
		// enters the plan-owned scheduler; it runs no network wave and
		// invents no acceptance event (F-21).
		if _, err := store.Current(); err != nil {
			return fmt.Errorf("read current network state: %w", err)
		}
	} else {
		snapshot, err := store.Refresh(ctx)
		if err != nil {
			return fmt.Errorf("refresh network state: %w", err)
		}
		// The accepted-wave event belongs solely to the branch that
		// actually ran one selected Direct-Origin Source wave.
		if err := events.encode(struct {
			Schema             string    `json:"schema"`
			Kind               string    `json:"kind"`
			At                 time.Time `json:"at"`
			Generation         string    `json:"generation"`
			Epoch              uint64    `json:"epoch"`
			SourceAttempts     uint16    `json:"source_attempts"`
			SourceOutcomes     [4]string `json:"source_outcomes"`
			LatestCompleteness string    `json:"latest_completeness"`
		}{"ardents-source-event-v1", "source-wave-accepted", time.Now().UTC(), snapshot.Generation, snapshot.Epoch,
			snapshot.SourceAttempts, snapshot.SourceOutcomes, snapshot.LatestCompleteness}); err != nil {
			return err
		}
	}
	if once || config.AutomaticRefreshInterval == 0 {
		return nil
	}
	return store.Wait(ctx)
}
func readSourcePlan(root, path string) (state.Config, error) {
	raw, err := readOperatorInput(path, 32<<10)
	if err != nil {
		return state.Config{}, fmt.Errorf("decode source plan: %w", err)
	}
	plan, err := runtimeplan.DecodeSource(raw)
	if err != nil {
		return state.Config{}, fmt.Errorf("decode source plan: %w", err)
	}
	config := state.Config{Root: root, LocalRoleStateRoot: plan.LocalRoleStateRoot, Threshold: plan.Threshold,
		NetworkID: plan.NetworkID, Authorities: plan.Authorities,
		Source: source.Config{MaterialIndex: plan.MaterializationIndex, OrderSeed: plan.OrderSeed}, RuntimeProfile: plan.RuntimeProfile,
		AutomaticRefreshInterval: time.Duration(plan.RefreshIntervalMS) * time.Millisecond,
		Clock:                    time.Now, ClockObservationFile: plan.ClockObservationFile, ClockObservation: plan.ClockObservation}
	for index, member := range plan.Sources {
		declared := &config.Source.Sources[index]
		declared.Address, declared.ServerName = member.Address, member.ServerName
		declared.Family, declared.EndpointHandle = member.Family, member.EndpointHandle
		declared.Identity, declared.LeafKeyDigest = plan.Identities[index], plan.LeafKeyDigests[index]
	}
	if err := loadSourceCredentials(&config, plan.Source); err != nil {
		return config, err
	}
	return config, nil
}
