package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
)

type options struct {
	action   string
	provider string
	topic    string
	payload  string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "ardents-store-probe-")
	if err != nil {
		return fmt.Errorf("create probe state: %w", err)
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := newClient(dir)
	client.SetBootstrapNodes([]string{opts.provider})
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start constrained client: %w", err)
	}
	defer client.Stop(context.Background())

	envelope := networkprivacy.SealedEnvelope{
		PubsubTopic:  networkapi.DefaultPubsubTopic,
		ContentTopic: opts.topic,
		Payload:      []byte(opts.payload),
	}
	if opts.action == "publish" {
		if err := client.PublishPrivateLightpush(ctx, opts.provider, envelope); err != nil {
			return err
		}
		return fetchRetained(ctx, client, opts.provider, envelope)
	}
	return fetchRetained(ctx, client, opts.provider, envelope)
}

func parseOptions(args []string) (options, error) {
	set := flag.NewFlagSet("ard-store-probe", flag.ContinueOnError)
	action := set.String("action", "", "publish or fetch")
	provider := set.String("provider", "", "provider multiaddress")
	topic := set.String("topic", "", "opaque content topic")
	payload := set.String("payload", "", "opaque test payload")
	if err := set.Parse(args); err != nil {
		return options{}, err
	}
	if (*action != "publish" && *action != "fetch") || *provider == "" || *topic == "" || *payload == "" {
		return options{}, fmt.Errorf("action, provider, topic, and payload are required")
	}
	return options{action: *action, provider: *provider, topic: *topic, payload: *payload}, nil
}

func newClient(dir string) networkapi.Service {
	return networkapi.New(networkapi.Config{
		NodeProfile:      networkapi.NodeProfileConstrainedClient,
		Profile:          networkapi.ProfileTCPOnly,
		ReachabilityMode: networkapi.ReachabilityOutboundOnly,
		StorePath:        filepath.Join(dir, "waku-store.db"),
		PrivateKeyPath:   filepath.Join(dir, "waku-key.json"),
	})
}

func fetchRetained(ctx context.Context, client networkapi.Service, provider string, expected networkprivacy.SealedEnvelope) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		items, err := client.FetchPrivateEnvelopes(ctx, []string{provider}, expected.ContentTopic)
		if err == nil {
			for _, item := range items {
				if bytes.Equal(item.Payload, expected.Payload) {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("retained opaque envelope not recovered: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
