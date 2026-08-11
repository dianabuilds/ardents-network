package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification"
)

func run(arguments []string, output, diagnostics io.Writer) int {
	result, err := evaluate(arguments)
	if err != nil {
		result = qualification.Result{Verdict: "invalid", Reason: err.Error()}
	}
	if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
		fmt.Fprintln(diagnostics, encodeErr)
		return 2
	}
	switch result.Verdict {
	case "pass":
		return 0
	case "fail":
		return 1
	default:
		return 2
	}
}

func evaluate(arguments []string) (qualification.Result, error) {
	if len(arguments) == 0 || arguments[0] != "offline" {
		return qualification.Result{}, errors.New("usage: ardents-qualify offline [flags]")
	}
	flags := flag.NewFlagSet("offline", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var root, networkHex, authorityHex, atText, materialsText string
	var threshold int
	flags.StringVar(&root, "state-root", "", "candidate state root")
	flags.StringVar(&networkHex, "network-id", "", "32-byte network identity in hex")
	flags.StringVar(&authorityHex, "authorities", "", "comma-separated Ed25519 public keys in hex")
	flags.IntVar(&threshold, "threshold", 0, "signature threshold")
	flags.StringVar(&atText, "at", "", "verification time in RFC3339")
	flags.StringVar(&materialsText, "materializations", "", "comma-separated materialization files")
	if err := flags.Parse(arguments[1:]); err != nil {
		return qualification.Result{}, err
	}
	if flags.NArg() != 0 {
		return qualification.Result{}, errors.New("offline qualification has unexpected positional arguments")
	}
	networkID, authorities, at, materials, err := parseCaseInputs(networkHex, authorityHex, atText, materialsText)
	if err != nil {
		return qualification.Result{}, err
	}
	return qualification.VerifyOffline(qualification.OfflineCase{
		Root: root, NetworkID: networkID, Authorities: authorities,
		Threshold: threshold, Now: at, Materializations: materials,
	}), nil
}

func parseCaseInputs(networkHex, authorityHex, atText, materialsText string) ([32]byte, map[[32]byte]ed25519.PublicKey, time.Time, [][]byte, error) {
	var networkID [32]byte
	if err := decodeHex(networkHex, networkID[:]); err != nil {
		return networkID, nil, time.Time{}, nil, fmt.Errorf("network-id: %w", err)
	}
	authorities := make(map[[32]byte]ed25519.PublicKey)
	for _, encoded := range strings.Split(authorityHex, ",") {
		public := make([]byte, ed25519.PublicKeySize)
		if err := decodeHex(encoded, public); err != nil {
			return networkID, nil, time.Time{}, nil, fmt.Errorf("authority: %w", err)
		}
		authorities[sha256.Sum256(public)] = ed25519.PublicKey(public)
	}
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil {
		return networkID, nil, time.Time{}, nil, fmt.Errorf("at: %w", err)
	}
	materials, err := readMaterials(materialsText)
	return networkID, authorities, at, materials, err
}

func readMaterials(paths string) ([][]byte, error) {
	if paths == "" {
		return nil, errors.New("materializations are required")
	}
	parts := strings.Split(paths, ",")
	if len(parts) > 64 {
		return nil, errors.New("too many materializations")
	}
	materials := make([][]byte, len(parts))
	for index, path := range parts {
		var err error
		materials[index], err = readQualifierFile(path, 35<<10)
		if err != nil {
			return nil, fmt.Errorf("read materialization %d: %w", index, err)
		}
	}
	return materials, nil
}

func decodeHex(encoded string, destination []byte) error {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(decoded) != len(destination) {
		return fmt.Errorf("decoded length is %d, want %d", len(decoded), len(destination))
	}
	copy(destination, decoded)
	return nil
}
