package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

type sourcePlan struct {
	Schema               string             `json:"schema"`
	NetworkID            string             `json:"network_id"`
	AuthorityPublic      []string           `json:"authority_public"`
	Threshold            int                `json:"threshold"`
	ClockObservedAt      string             `json:"clock_observed_at"`
	OrderSeed            string             `json:"order_seed"`
	MaterializationIndex uint32             `json:"materialization_index"`
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
	root, planPath := "", ""
	flags.StringVar(&root, "state-root", "", "owned state root")
	flags.StringVar(&planPath, "source-plan", "", "bounded source plan JSON")
	if err := flags.Parse(arguments[1:]); err != nil || flags.NArg() != 0 {
		return errors.New("usage: ardents refresh-sources --state-root PATH --source-plan PATH")
	}
	config, err := readSourcePlan(root, planPath)
	if err != nil {
		return err
	}
	store, err := networkstate.Open(config)
	if err != nil {
		return fmt.Errorf("open network state: %w", err)
	}
	defer store.Close()
	snapshot, err := store.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("refresh network state: %w", err)
	}
	return json.NewEncoder(output).Encode(struct {
		Schema             string    `json:"schema"`
		Kind               string    `json:"kind"`
		Generation         string    `json:"generation"`
		Epoch              uint64    `json:"epoch"`
		SourceAttempts     uint16    `json:"source_attempts"`
		SourceOutcomes     [4]string `json:"source_outcomes"`
		LatestCompleteness string    `json:"latest_completeness"`
	}{"ardents-h3-s1-source-event-v1", "source-wave-accepted", snapshot.Generation, snapshot.Epoch,
		snapshot.SourceAttempts, snapshot.SourceOutcomes, snapshot.LatestCompleteness})
}

func readSourcePlan(root, path string) (networkstate.Config, error) {
	raw, err := readCommandFile(path, 32<<10)
	if err != nil {
		return networkstate.Config{}, fmt.Errorf("read source plan: %w", err)
	}
	var plan sourcePlan
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil || plan.Schema != "ardents-h3-source-plan-v1" ||
		len(plan.Sources) != 2 || len(plan.AuthorityPublic) == 0 || len(plan.AuthorityPublic) > 16 {
		return networkstate.Config{}, errors.New("source plan is not canonical or complete")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return networkstate.Config{}, errors.New("source plan contains trailing JSON")
	}
	config := networkstate.Config{Root: root, Threshold: plan.Threshold, Authorities: make(map[[32]byte]ed25519.PublicKey), SourceMaterializationIndex: plan.MaterializationIndex}
	if err := decodeFixedHex(plan.NetworkID, config.NetworkID[:]); err != nil {
		return config, err
	}
	for _, encoded := range plan.AuthorityPublic {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeFixedHex(encoded, public); err != nil {
			return config, err
		}
		config.Authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	config.Clock = time.Now
	if config.ClockObservation, err = time.Parse(time.RFC3339, plan.ClockObservedAt); err != nil {
		return config, err
	}
	if err := decodeFixedHex(plan.OrderSeed, config.SourceOrderSeed[:]); err != nil {
		return config, err
	}
	if err := loadSourceCredentials(&config, plan); err != nil {
		return config, err
	}
	return config, nil
}
