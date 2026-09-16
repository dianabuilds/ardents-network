//go:build linux

package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("qualification-network", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var config fixtureConfig
	var at string
	flags.StringVar(&config.Output, "output", "", "new private fixture directory")
	flags.StringVar(&config.ReaderHost, "reader-host", "", "reader host IPv4")
	flags.StringVar(&config.PublisherHost, "publisher-host", "", "publisher host IPv4")
	flags.StringVar(&config.Carrier, "carrier", "", "tcp-tls or quic")
	flags.StringVar(&config.Cell, "cell", "", "NET-14 cell")
	flags.StringVar(&config.Profile, "profile", "", "client-to-publisher or publisher-to-client")
	flags.StringVar(&config.Seed, "seed", "", "32-byte lowercase hex fixture seed")
	flags.StringVar(&at, "at", "", "canonical UTC RFC3339 fixture start")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return errors.New("qualification network arguments are invalid")
	}
	var err error
	config.At, err = time.Parse(time.RFC3339, at)
	if err != nil || config.At.Format(time.RFC3339) != at || config.At.Location() != time.UTC ||
		config.At.Nanosecond() != 0 {
		return errors.New("fixture time must be canonical UTC RFC3339")
	}
	seed, err := hex.DecodeString(config.Seed)
	if err != nil || len(seed) != 32 || hex.EncodeToString(seed) != config.Seed {
		return errors.New("fixture seed must be canonical lowercase SHA-256-width hex")
	}
	if config.Carrier != "tcp-tls" && config.Carrier != "quic" {
		return errors.New("fixture carrier is invalid")
	}
	switch config.Cell {
	case "net14ad", "net14s", "net14-recovery":
	default:
		return errors.New("fixture cell is invalid")
	}
	switch config.Profile {
	case "client-to-publisher", "publisher-to-client":
	default:
		return errors.New("fixture workload profile is invalid")
	}
	if net.ParseIP(config.ReaderHost).To4() == nil || net.ParseIP(config.PublisherHost).To4() == nil ||
		config.ReaderHost == config.PublisherHost {
		return errors.New("fixture requires two distinct literal IPv4 hosts")
	}
	config.Output, err = filepath.Abs(config.Output)
	if err != nil || filepath.Clean(config.Output) != config.Output {
		return errors.New("fixture output path is invalid")
	}
	if err := os.Mkdir(config.Output, 0o700); err != nil {
		return fmt.Errorf("create new fixture output: %w", err)
	}
	if err := buildFixture(config); err != nil {
		return err
	}
	return nil
}
