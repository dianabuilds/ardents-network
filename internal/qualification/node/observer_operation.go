package node

import (
	"context"
	"errors"
	"io"
	"time"
)

// RunObserver executes one bounded candidate-image observer operation. It
// returns handled=false when arguments belong to the campaign command surface.
func RunObserver(ctx context.Context, arguments []string, input io.Reader, output io.Writer) (bool, error) {
	if len(arguments) == 0 {
		return false, nil
	}
	command, values := arguments[0], arguments[1:]
	switch command {
	case "collector-node":
		if len(values) != 0 {
			return true, errors.New("collector-node has unexpected arguments")
		}
		for {
			time.Sleep(24 * time.Hour)
		}
	case "sample-node-stream":
		if len(values) != 1 {
			return true, errors.New("sample-node-stream requires one candidate set")
		}
		return true, streamHostResourceOutput(ctx, input, output, values[0])
	case "sample-node":
		if len(values) > 1 {
			return true, errors.New("sample-node has unexpected arguments")
		}
		encoded := ""
		if len(values) == 1 {
			encoded = values[0]
		}
		raw, err := sampleResources(time.Now(), encoded)
		return true, writeNodeObserverOutput(output, raw, err)
	case "build-info-node":
		if len(values) != 0 {
			return true, errors.New("build-info-node has unexpected arguments")
		}
		raw, err := readCandidateBuildIdentity([]string{
			"/usr/local/bin/ardents", "/usr/local/bin/ardents-node", "/usr/local/bin/ardents-qualify",
		})
		return true, writeNodeObserverOutput(output, raw, err)
	default:
		return false, nil
	}
}

func sampleResources(at time.Time, encoded string) ([]byte, error) {
	if encoded == "" {
		return sampleContainerResources(at)
	}
	return sampleHostResources(at, encoded)
}

func writeNodeObserverOutput(output io.Writer, raw []byte, err error) error {
	if err != nil {
		return err
	}
	written, err := output.Write(append(raw, '\n'))
	if err == nil && written != len(raw)+1 {
		return io.ErrShortWrite
	}
	return err
}
