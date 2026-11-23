package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// Local command facts are explicit operator assertions, never Network authority.
// Reopening the file at each boundary permits withdrawal without cached approval.
func admissionObserver(path string) func() (admission.AuthorityFacts, time.Time, error) {
	return func() (admission.AuthorityFacts, time.Time, error) {
		var p admission.AuthorityFacts
		raw, err := readBounded(path, 64<<10)
		if err == nil {
			err = decodeAdmissionObject(raw, &p)
		}
		now := time.Now().UTC()
		if err == nil {
			err = p.ValidateAt(now)
		}
		return p, now, err
	}
}

func decodeAdmissionObject(raw []byte, value any) error { return artifacts.DecodeObject(raw, value) }

func admissionConfig(args []string, value any) error {
	return admissionConfigBounded(args, value, 64<<10)
}
func admissionConfigBounded(args []string, value any, maximum int64) error {
	if len(args) != 2 || args[0] != "--config" {
		return errors.New("invalid arguments")
	}
	raw, err := readBounded(args[1], maximum)
	if err != nil {
		return err
	}
	return decodeAdmissionObject(raw, value)
}

func absoluteAdmissionPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}

// The command owns its input/output descriptors. Closing them on cancellation
// joins the blocking local I/O; no abandoned scanner goroutine is created.
func admissionConsole(ctx context.Context, in io.ReadCloser, out io.Writer, step func(context.Context, []byte) (any, bool, error)) int {
	if ctx == nil || in == nil {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	joined := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = in.Close()
		if closer, ok := out.(io.Closer); ok {
			_ = closer.Close()
		}
		close(joined)
	})
	defer func() {
		if !stop() {
			<-joined
		}
	}()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	encode := json.NewEncoder(out)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return 130
		}
		value, done, err := step(ctx, scanner.Bytes())
		if err != nil && value == nil {
			value = map[string]string{"outcome": "refused"}
		}
		if encode.Encode(value) != nil {
			return 2
		}
		if done {
			if err != nil {
				return 1
			}
			return 0
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	if scanner.Err() != nil {
		return 2
	}
	return 0
}

func runAdmissionLocal(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if operation == "holder" || operation == "receiver" || operation == "execution-holder" || operation == "execution-holder-live" {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		input, err := admissionConsoleFile(os.Stdin)
		if err != nil {
			return 1
		}
		files := []*os.File{input}
		closeFiles := func() {
			for _, f := range files {
				_ = f.Close()
			}
		}
		defer closeFiles()
		writers := []io.Writer{out, diagnostic}
		for i, w := range writers {
			if file, ok := w.(*os.File); ok {
				owned, err := admissionConsoleFile(file)
				if err != nil {
					return 1
				}
				files = append(files, owned)
				writers[i] = owned
			}
		}
		joined := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { closeFiles(); close(joined) })
		defer func() {
			if !stop() {
				<-joined
			}
		}()
		if operation == "holder" {
			return runAdmissionHolder(ctx, args, input, writers[0], writers[1])
		}
		if operation == "execution-holder" {
			return runExecutionPreparation(ctx, args, input, writers[0], writers[1])
		}
		if operation == "execution-holder-live" {
			return runExecutionRouteHolder(ctx, args, input, writers[0], writers[1])
		}
		return runAdmissionReceiver(ctx, args, input, writers[0], writers[1])
	}
	switch operation {
	case "allocate":
		return runAdmissionAllocation(ctx, args, out)
	case "issue-current":
		return runAdmissionCurrentIssuer(ctx, args, out)
	}
	return 2
}
