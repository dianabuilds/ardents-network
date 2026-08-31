package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/browser/adapter"
)

type plan struct {
	Schema                string `json:"schema"`
	ApplicationSocket     string `json:"application_socket"`
	BrowserEntryStatePath string `json:"browser_entry_state_path"`
}

type event struct {
	Kind string `json:"kind"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if ctx == nil || len(arguments) != 2 || arguments[0] != "run" || arguments[1] == "" || output == nil {
		return errors.New("usage: ardents-browser run <browser-adapter.json>")
	}
	input, err := readPlan(arguments[1])
	if err != nil {
		return err
	}
	runtime, err := browseradapter.Open(ctx, browseradapter.Config{ApplicationSocket: input.ApplicationSocket,
		BrowserEntryStatePath: input.BrowserEntryStatePath})
	if err != nil {
		return err
	}
	defer runtime.Close()
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(event{Kind: "browser-adapter-ready"}); err != nil {
		return err
	}
	<-ctx.Done()
	if err := runtime.Close(); err != nil {
		return err
	}
	return encoder.Encode(event{Kind: "browser-adapter-stopped"})
}

func readPlan(path string) (plan, error) {
	var input plan
	file, err := os.Open(path)
	if err != nil {
		return input, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return plan{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return plan{}, errors.New("browser Adapter plan has trailing data")
	}
	if input.Schema != "ardents-browser-adapter-v1" || !filepath.IsAbs(input.ApplicationSocket) || !filepath.IsAbs(input.BrowserEntryStatePath) ||
		bytes.Equal([]byte(input.ApplicationSocket), []byte(input.BrowserEntryStatePath)) {
		return plan{}, errors.New("browser Adapter plan is invalid")
	}
	return input, nil
}
