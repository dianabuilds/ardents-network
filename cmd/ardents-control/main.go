package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const commandUsage = "usage: ardents-control source, forwarding, join, descriptor, prepare-closed-profile, sign-closed-profile, inspect-closed-profile, or inspect-closed-issuer-profile"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return errors.New(commandUsage)
	}
	switch arguments[0] {
	case "descriptor":
		return runDescriptorControl(arguments[1:], output)
	case "join":
		return runJoinControl(arguments[1:], output)
	case "forwarding":
		return runForwardingControl(arguments[1:], output)
	case "source":
		return runSourceControl(arguments[1:], output)
	// The ACA1 disclosure reader and its qualification preparation are retired
	// (ADR-0127, #647): successor per-domain commands verify the same authority
	// facts, and no unified replacement report is selected. Each route refuses
	// before parsing arguments or opening any file, root, or floor.
	case "inspect-bundle":
		return errors.New("inspect-bundle is retired")
	case "inspect-transitions":
		return errors.New("inspect-transitions is retired")
	case "prepare-qualification-evidence":
		return errors.New("prepare-qualification-evidence is retired")
	case "prepare-qualification-catalog":
		return errors.New("prepare-qualification-catalog is retired")
	case "prepare-qualification-node-record":
		return errors.New("prepare-qualification-node-record is retired; use ardents-next network prepare-initial-node-record")
	case "prepare-qualification-epoch":
		return errors.New("prepare-qualification-epoch is retired; use ardents-next network prepare-initial-epoch")
	case "inspect-alpha-corpus":
		return errors.New("inspect-alpha-corpus is retired")
	case "accept-alpha-corpus":
		return errors.New("accept-alpha-corpus is retired")
	case "prepare-closed-profile":
		return prepareClosedProfile(arguments[1:], output)
	case "sign-closed-profile":
		return signClosedProfile(arguments[1:], output)
	case "inspect-closed-issuer-profile":
		return inspectClosedIssuerProfile(arguments[1:], output)
	case "inspect-closed-profile":
		return inspectClosedProfile(arguments[1:], output)
	default:
		return errors.New(commandUsage)
	}
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize || hex.EncodeToString(raw) != encoded {
		return nil, errors.New("alpha control disclosure key is invalid")
	}
	return ed25519.PublicKey(raw), nil
}

func decodeIdentifier(encoded string) ([32]byte, error) {
	var result [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(result) || hex.EncodeToString(raw) != encoded {
		return result, errors.New("alpha control identifier is invalid")
	}
	copy(result[:], raw)
	return result, nil
}

func readControlFile(path string, maximum int64) ([]byte, error) {
	if path == "" {
		return nil, errors.New("control file is required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > maximum {
		return nil, errors.New("control file is not a bounded regular file")
	}
	return os.ReadFile(path)
}
