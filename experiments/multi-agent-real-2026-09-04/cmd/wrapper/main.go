//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	if len(arguments) == 0 {
		return usageError()
	}
	switch arguments[0] {
	case "prepare":
		if len(arguments) != 2 {
			return usageError()
		}
		manifestPath, _, err := prepareRun(arguments[1], time.Now, nil)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, manifestPath)
		return err
	case "prompt":
		if len(arguments) != 3 {
			return usageError()
		}
		manifest, err := loadManifest(arguments[1])
		if err != nil {
			return err
		}
		if _, ok := manifest.Personas[arguments[2]]; !ok {
			return errors.New("persona is not owned by this run")
		}
		_, err = fmt.Fprint(output, personaPrompt(arguments[2]))
		return err
	case "act":
		if len(arguments) != 4 {
			return usageError()
		}
		manifest, err := loadManifest(arguments[1])
		if err != nil {
			return err
		}
		var raw []byte
		if arguments[3] == "-" {
			raw, err = readBoundedReader(os.Stdin, 4<<10)
		} else {
			raw, err = readBounded(arguments[3], 4<<10)
		}
		if err != nil {
			return err
		}
		request, err := decodeAction(raw)
		if err != nil {
			return err
		}
		name, _, actionErr := recordAction(manifest, arguments[2], request, dockerRunner{}, time.Now)
		if name != "" {
			if _, err := fmt.Fprintln(output, name); err != nil {
				return err
			}
		}
		return actionErr
	case "verify":
		if len(arguments) != 2 {
			return usageError()
		}
		manifest, err := loadManifest(arguments[1])
		if err != nil {
			return err
		}
		summary, err := verifyRun(manifest)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	case "self-test":
		if len(arguments) != 1 {
			return usageError()
		}
		if err := runSelfTest(); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "self-test: PASS")
		return err
	default:
		return usageError()
	}
}

func readBounded(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readBoundedReader(file, limit)
}

func readBoundedReader(reader io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || int64(len(raw)) > limit {
		return nil, errors.New("input file is empty or exceeds its bound")
	}
	return raw, nil
}

func usageError() error {
	return errors.New("usage: wrapper prepare EVIDENCE_ROOT | prompt MANIFEST PERSONA | act MANIFEST PERSONA ACTION_FILE | verify MANIFEST | self-test")
}
