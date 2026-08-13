package node

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type nodeResourceStreamUpdate struct {
	Generation uint64              `json:"generation"`
	Candidates []nodeHostCandidate `json:"candidates"`
}

type nodeResourceStreamOutput struct {
	Generation uint64          `json:"generation"`
	Samples    json.RawMessage `json:"samples"`
}

func streamHostResourceOutput(ctx context.Context, input io.Reader, output io.Writer, encoded string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	updates := make(chan string, 1)
	go scanNodeResourceUpdates(ctx, input, updates)
	current, err := decodeNodeResourceStreamUpdate(encoded)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case update, open := <-updates:
			if !open {
				return nil
			}
			current, err = decodeNodeResourceStreamUpdate(update)
			if err != nil {
				return err
			}
		case at := <-ticker.C:
			candidates, err := json.Marshal(current.Candidates)
			if err != nil {
				return err
			}
			samples, err := sampleHostResources(at, string(candidates))
			if err != nil {
				return err
			}
			raw, err := json.Marshal(nodeResourceStreamOutput{Generation: current.Generation, Samples: samples})
			if err != nil {
				return err
			}
			if err := writeNodeObserverOutput(output, raw, nil); err != nil {
				return err
			}
		}
	}
}

func encodeNodeResourceStreamUpdate(generation uint64, candidates []nodeHostCandidate) ([]byte, error) {
	if generation == 0 {
		return nil, errors.New("node resource stream generation is invalid")
	}
	payload, err := json.Marshal(nodeResourceStreamUpdate{Generation: generation, Candidates: candidates})
	if err != nil || len(payload) > 4096 {
		return nil, errors.Join(err, errors.New("node resource stream update exceeds its bound"))
	}
	return payload, nil
}

func decodeNodeResourceStreamUpdate(encoded string) (nodeResourceStreamUpdate, error) {
	var update nodeResourceStreamUpdate
	if len(encoded) == 0 || len(encoded) > 4096 || json.Unmarshal([]byte(encoded), &update) != nil ||
		update.Generation == 0 || len(update.Candidates) < 1 || len(update.Candidates) > 5 {
		return update, errors.New("node resource stream update is invalid")
	}
	return update, nil
}

func decodeNodeResourceStreamOutput(raw []byte) (nodeResourceStreamOutput, []nodeResourceSnapshot, error) {
	var output nodeResourceStreamOutput
	if err := json.Unmarshal(raw, &output); err != nil || output.Generation == 0 || len(output.Samples) == 0 {
		return output, nil, invalidNodeCampaign(errors.Join(err, errors.New("node resource stream output is invalid")))
	}
	samples, err := decodeNodeResourceSamples(output.Samples, 5)
	return output, samples, err
}

func scanNodeResourceUpdates(ctx context.Context, input io.Reader, updates chan<- string) {
	defer close(updates)
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4096)
	for scanner.Scan() {
		select {
		case updates <- scanner.Text():
		case <-ctx.Done():
			return
		}
	}
}

func streamHostResources(ctx context.Context, output io.Writer, schedule <-chan time.Time,
	sample func(time.Time) ([]byte, error),
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case at := <-schedule:
			raw, err := sample(at)
			if err != nil {
				return err
			}
			written, err := output.Write(append(raw, '\n'))
			if err != nil {
				return err
			}
			if written != len(raw)+1 {
				return io.ErrShortWrite
			}
		}
	}
}
